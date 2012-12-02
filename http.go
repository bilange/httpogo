/*  

ATTENTION: a *chaque* fois qu'on compile le programme et qu'on ecrase le
fichier executable d'origine où le serveur sera exécuté, on DOIT ABSOLUMENT
faire la commande suivante en tant que root:

	setcap cap_net_bind_service=+ep /chemin/vers/fichier/compilé

Ceci nous donne la possibilité d'ouvrir un port TCP < 1024 en tant qu'un
usager UNIX non-root.

*/

// TODO: Support pour .bin et/ou .cgi dans le dossier comme office de 
//       index.html??
// TODO: Configurer php.ini (via PHP_INI_PATH), et dans php.ini configurer
//       doc_root dans le php.ini: http://ca2.php.net/manual/en/ini.core.php#ini.doc-root

package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/russross/blackfriday" //markdown
	"io/ioutil"
	"mime"
	"net/http"
	"net/http/cgi"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	LOG_DEBUG   = 1
	LOG_INFO    = 2
	LOG_WARNING = 4
	LOG_ERROR   = 8
)

const (
	ERR_LOG_SILENT = 1
	ERR_LOG_FILE   = 2
	ERR_LOG_STDOUT = 4
)

var port int = 80                        // Default TCP Port
var workingDirectory string = "/var/www" // Dir. where files and v-host dirs are stored
var defaultVHost string = "public_html"  // Default virtual host if no host matches
var loggingEnabled bool = true           // Activate logging?
var runAsRoot bool = false               // Permettre de rouler sous root?

var errLoggingOutput int = ERR_LOG_STDOUT //	Where should we print out errors/debug?
var errLoggingLevel int = LOG_INFO        // Minimal logging

var hiddenFiles []string = []string{ // Files we NEVER want shown
	".auth",
	".bin",
	".cgi",
}

//commandline parameters: 
// see use -h for a full parameter listing.
func main() {
	absoluteWd, _ := os.Getwd() //par defaut, le dossier contenant l'executable servira de workingDirectory.

	parsedPort := flag.Int("port", 80, "TCP Port the server will listen onto")
	parsedWorkingDirectory := flag.String("root", absoluteWd, "Root directory (where binaries, scripts, v-host public folders are stored)")
	parsedDefaultVHost := flag.String("webdir", "public_html", "Default V-Host, in the case where no -root subfolders matches the requested HTTP Host")
	parsedLog := flag.Bool("log", false, "Enable logging")
	parsedRunAsRoot := flag.Bool("runasroot", false, "Allows execution of this program under root")
	parsedLogLevel := flag.String("loglevel", "error", "What minimum logging verbosity should logging use? Choices: DEBUG|INFO|WARNING|ERROR")
	parsedErrLog := flag.String("errorto", "stdout", "Where should errors be logged? Choices: SILENT|STDOUT|FILE")

	flag.Parse()

	port = *parsedPort
	workingDirectory = *parsedWorkingDirectory
	defaultVHost = *parsedDefaultVHost
	loggingEnabled = *parsedLog

	if os.Getuid() == 0 && *parsedRunAsRoot == false {
		println("Refusing to run as root. If this is REALLY what you want, append -runasroot=true to your command line.")
		os.Exit(1)
	}

	// MINIMAL ERROR LEVEL LOGGING:
	switch {
	case *parsedLogLevel == "error":
		errLoggingLevel = LOG_ERROR
	case *parsedLogLevel == "warning":
		errLoggingLevel = LOG_WARNING
	case *parsedLogLevel == "info":
		errLoggingLevel = LOG_INFO
	case *parsedLogLevel == "debug":
		errLoggingLevel = LOG_DEBUG
	}

	// ERROR LOGING OUTPUT
	switch {
	case *parsedErrLog == "silent":
		errLoggingOutput = ERR_LOG_SILENT
	case *parsedErrLog == "stdout":
		errLoggingOutput = ERR_LOG_STDOUT
	case *parsedErrLog == "file":
		errLoggingOutput = ERR_LOG_FILE
	}

	http.HandleFunc("/", requestHandler)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		fmt.Printf("Erreur: %s\n", err.Error())
	}
	return
}

//requestHandler takes change of the incoming connection, by looking for a
//matching vhost as a subfolder of -root . After the right vhost is found,
//(or uses the defaultVHost variable as a last case scenario), the request is
//then handled differently depending whether this is a static file, a binary,
//a PHP script, a markdown document, or an open folder with no index files.
func requestHandler(w http.ResponseWriter, r *http.Request) {

	pwd := workingDirectory
	hostSplit := strings.Split(r.Host, ":")
	host := hostSplit[0]

	vHostFolder := path.Join(pwd, host)
	errorLog(LOG_DEBUG, host, fmt.Sprintf("vHost Folder (before checks): %s", vHostFolder))

	vHostDirExists, _ := fileIsDir(vHostFolder)
	if vHostDirExists == true {
		pwd = vHostFolder
	} else {
		pwd = path.Join(pwd, defaultVHost) //Fallback Default. 
		vHostFolder = pwd
	}

	errorLog(LOG_DEBUG, host, fmt.Sprintf("vHost Folder (after checks): %s", vHostFolder))

	fileAbsolute := filepath.Join(pwd, r.URL.Path)

	errorLog(LOG_DEBUG, host, fmt.Sprintf("%s:%s -> %s ", host, r.URL.Path, fileAbsolute))

	if fileIsDiscarded(r.URL.Path) {
		errorLog(LOG_WARNING, host, fmt.Sprintf("Filename '%s' is returned to the client as '404 not found' due to being used internally by the server. If this is a legitimate file, change the file name to something else. ", fileAbsolute))
		accessLog(vHostFolder, r, 404)
		fileNotFoundHandler(w, r)
		return
	}

	//For not found files (exception: markdown 'source' files that are later
	//passed with the original unparsed .md file)
	fexists, _ := fileExists(fileAbsolute)
	if fexists == false && !strings.HasSuffix(r.URL.Path, ".md.txt") {
		accessLog(vHostFolder, r, 404)
		fileNotFoundHandler(w, r)
		return
	}

	authFile := needsAuth(vHostFolder, r.URL.Path)
	authPassed := false
	if authFile != "" {
		//The user must be logged to see the folder's contents. An user/password box will be shown on-screen.
		//The user will stay logged on until the user closes the browser window or requests another folder on the same hosts
		//requiring ANOTHER set of user/password pair.
		//See : http://en.wikipedia.org/wiki/Basic_access_authentication
		//
		// The '.auth' file contains this on a single line
		//     USERNAME:PASSWORD
		// (Caution: the password is kept in clear-text!!!)
		userAuth := r.Header["Authorization"]
		if userAuth == nil { //User didn't provide a user/password pair for a folder requiring authentication
			requireHttpAuth(w, r, fmt.Sprintf("Basic realm=\"%s\"", strings.Replace(filepath.Dir(authFile), vHostFolder, "", -1)))
			accessLog(vHostFolder, r, 401)
			return
		} else {
			userAuthParts := strings.Split(userAuth[0], " ")
			if len(userAuthParts) == 2 {
				userAuthEncoded := userAuthParts[1]
				userAuthDecoded := fromBase64(userAuthEncoded)
				if !fileContainsLine(authFile, userAuthDecoded) { // .auth file doesnt contain the user:password pair the user provided
					requireHttpAuth(w, r, fmt.Sprintf("Basic realm=\"%s\"", strings.Replace(filepath.Dir(authFile), vHostFolder, "", -1)))
					accessLog(vHostFolder, r, 401)
					return
				} else {
					authPassed = true
					// User has been authenticated, letting the request fly
					// through the rest of the function body.
				}
			} else { //Bad auth HTTP request (userAuthParts should contain a base64-encoded user:password value as its 2nd element)
				requireHttpAuth(w, r, fmt.Sprintf("Basic realm=\"%s\"", strings.Replace(filepath.Dir(authFile), vHostFolder, "", -1)))
				accessLog(vHostFolder, r, 401)
				return
			}
		}
	}

	if authFile == "" {
		errorLog(LOG_DEBUG, host, "No auth found, carrying on.")
	} else {
		if authPassed == true {
			errorLog(LOG_DEBUG, host, "User successfully passed authentication.")
		}
	}

	phpActuallyBinary := (r.URL.Path == "/backend.php" || r.URL.Path == "/cron.php") //hard-coded exceptions (Intranet)
	if strings.HasSuffix(r.URL.Path, ".php") == true && (!phpActuallyBinary) {       //PHP file. This requires php-cgi properly compiled and stored in the 'root'/files folder.
		phpHandler(w, r, r.URL.Path)
		return
	}

	fdir, _ := fileIsDir(fileAbsolute) // Requested URL points to a folder
	if fdir == true {
		fileAbsolute += string(os.PathSeparator)
	}

	mimeType := mime.TypeByExtension(filepath.Ext(fileAbsolute))
	fexecutable, _ := fileIsExecutable(fileAbsolute)

	switch {
	case strings.HasSuffix(r.URL.Path, ".md"):
		accessLog(vHostFolder, r, 200)
		markdownHandler(w, r, fileAbsolute, false)
		return
	case strings.HasSuffix(r.URL.Path, ".md.txt"):
		accessLog(vHostFolder, r, 200)
		markdownHandler(w, r, fileAbsolute, true)
		return
	case mimeType == "application/octet-stream",
		mimeType == "" && fexecutable == true,
		strings.HasPrefix(mimeType, "text/x-sh"), phpActuallyBinary:
		accessLog(vHostFolder, r, 200)
		executableHandler(w, r, r.URL.Path)
		return
	case strings.HasPrefix(mimeType, "image"),
		strings.HasPrefix(mimeType, "text"),
		strings.HasPrefix(mimeType, "video"), strings.HasPrefix(mimeType, "audio"),
		strings.HasPrefix(mimeType, "music"),
		strings.HasSuffix(r.URL.Path, ".js"), strings.HasSuffix(r.URL.Path, ".css"),
		mimeType == "application/xml", mimeType == "application/javascript":

		errorLog(LOG_DEBUG, host, fmt.Sprintf("Serving 'known' file format: %s", filepath.Join(vHostFolder, r.URL.Path)))
		accessLog(vHostFolder, r, 200)
		http.ServeFile(w, r, filepath.Join(vHostFolder, r.URL.Path))
		return
	default:
		if fdir == true {
			//HACK: This block deals with requests to folders WITHOUT a
			//trailing slash, causing issues with in-document HTML links to
			//images/css/js. For open folders, this caused links to be
			//pointing at the folder's parent too. We're blindly forwarding
			//the request to the slashed equivalent and be done with it.
			if r.URL.Path[len(r.URL.Path)-1] != '/' {
				w.Header().Add("Location", r.URL.Path+"/\n")
				w.WriteHeader(http.StatusMovedPermanently)
				return
			}

			//For the following blocks, deals with open directories known and
			//supported scenarios (.cgi, .bin, indexes).
			if ok, _ := fileExists(filepath.Join(fileAbsolute, ".cgi")); ok {
				accessLog(vHostFolder, r, 200)
				executableHandler(w, r, filepath.Join(r.URL.Path, ".cgi"))
				return
			}
			if ok, _ := fileExists(filepath.Join(fileAbsolute, ".bin")); ok {
				accessLog(vHostFolder, r, 200)
				executableHandler(w, r, filepath.Join(r.URL.Path, ".bin"))
				return
			}
			if ok, _ := fileExists(filepath.Join(fileAbsolute, "index.html")); ok {
				accessLog(vHostFolder, r, 200)
				http.ServeFile(w, r, filepath.Join(fileAbsolute, "index.html"))
				return
			}
			if ok, _ := fileExists(filepath.Join(fileAbsolute, "index.php")); ok {
				accessLog(vHostFolder, r, 200)
				phpHandler(w, r, filepath.Join(r.URL.Path, "index.php"))
				return
			}
			// At this point, this is simply an open directory without any indexes.
			// Showing it as-is.
			accessLog(vHostFolder, r, 200)
			directoryHandler(w, r, fileAbsolute)
			return
		} else {
			accessLog(vHostFolder, r, 200)
			http.ServeFile(w, r, filepath.Join(vHostFolder, r.URL.Path))
		}
	}
}

/*****************************************************************************
								HANDLERS
*****************************************************************************/

func directoryHandler(w http.ResponseWriter, req *http.Request, directory string) {
	w.Header().Add("Content-type", "text/html")

	var response bytes.Buffer

	template, err := ioutil.ReadFile(filepath.Join(workingDirectory, "files", "dirlist-template.html"))
	if err != nil {
		template = []byte(`<!DOCTYPE html><html><head><meta charset="UTF-8"><style>.dir {font-weight: bold;}</style></head> <body><h1>Index of <!--DIRNAME--></h1><hr /><!--BODY--> </body></html> `)
	}
	files, err := ioutil.ReadDir(directory)
	if err != nil {
		response.WriteString(fmt.Sprintf("Error opening folder%s: %s\n", directory, err.Error()))
		return
	}

	// Displaying folders first, then the files.
	for _, v := range files {
		if v.IsDir() {
			response.WriteString(fmt.Sprintf("<a class=\"dir\" href=\"%s\">%s</a><br />\n", req.URL.Path+v.Name()+"/", v.Name()+"/"))
		}
	}
	response.WriteString("<br />")
	for _, v := range files {
		if v.Name() == ".auth" {
			continue
		}
		if !v.IsDir() {
			response.WriteString(fmt.Sprintf("<a class=\"file\" href=\"%s\">%s</a><br />\n", req.URL.Path+v.Name(), v.Name()))
		}
	}

	// Template handling, then displaying the whole thing to the client.
	s := strings.Replace(string(template), "<!--BODY-->", response.String(), 1)
	s = strings.Replace(s, "<!--DIRNAME-->", req.URL.Path, 1)
	w.Write([]byte(s))
}

func markdownHandler(w http.ResponseWriter, req *http.Request, file string, printSource bool) {
	if printSource == false { // Markdown -> HTML
		template, err := ioutil.ReadFile(filepath.Join(workingDirectory, "files", "markdown-template.html"))
		if err != nil {
			template = []byte(`<!DOCTYPE html><html><head><meta charset="UTF-8"> </head> <body><!--BODY--> </body></html> `)
		}

		md, err := ioutil.ReadFile(file)
		if err == nil {
			output := blackfriday.MarkdownCommon(md)
			w.Write([]byte(strings.Replace(string(template), "<!--BODY-->", string(output), -1)))
		}
	} else {
		md, err := ioutil.ReadFile(file[0 : len(file)-4])
		if err == nil {
			w.Write([]byte(md))
		} else {
			w.Write([]byte("Unable to read markdown file."))
		}
	}
	return

}

//phpHandler deals with PHP files. Warning, this blindly assumes there's a 
//php-cgi file under the 'files' directory contained in the -root command
//argument. TODO.
func phpHandler(w http.ResponseWriter, req *http.Request, script string) {
	pwd := workingDirectory

	hostSplit := strings.Split(req.Host, ":")
	host := hostSplit[0]

	vHostFolder := path.Join(pwd, host)
	if ok, _ := fileIsDir(vHostFolder); ok {
		pwd = vHostFolder
	} else {
		pwd = path.Join(pwd, defaultVHost)
	}

	cgiHandler := cgi.Handler{
		Path: path.Join(workingDirectory, "files/php-cgi"), //TODO: Variable?
		Dir:  pwd,
		Root: pwd,
		Args: []string{req.URL.Path},
		Env: []string{
			"REDIRECT_STATUS=200",
			//original, working for dummy files:
			//"SCRIPT_FILENAME=" + path.Join(pwd, script),
			//"SCRIPT_NAME=" + path.Join(pwd, script),

			"SCRIPT_FILENAME=" + path.Join(pwd, script),
			"SCRIPT_NAME=" + script,
			//"PHP_SELF=" + script, //PHP automagically appends PHP_SELF's value
		},
	}
	errorLog(LOG_DEBUG, host, fmt.Sprintf("CGI Handler: %#v", cgiHandler))
	cgiHandler.ServeHTTP(w, req)
}

//executablehandler deals with self-contained executable files, like go
//compiled files, shell scripts and other programs we don't have the control
//ultimately. YOU are basically held responsible for the content. We're just
//doing the mailman at this rate.
//TODO: Pass HTTP GET parameters
func executableHandler(w http.ResponseWriter, req *http.Request, bin string) {
	pwd := workingDirectory

	hostSplit := strings.Split(req.Host, ":")
	host := hostSplit[0]

	vHostFolder := path.Join(pwd, host)
	vHostDirExists, _ := fileIsDir(vHostFolder)
	if vHostDirExists == true {
		pwd = vHostFolder
	} else {
		pwd = path.Join(pwd, defaultVHost)
	}
	cgiHandler := cgi.Handler{
		//Path: path.Join(pwd, req.URL.Path),
		Path: path.Join(pwd, bin),
		Dir:  pwd,
		Root: pwd,
		//Args: []string{file},
		//Env:  []string{"SCRIPT_FILENAME=" + file},
	}
	cgiHandler.ServeHTTP(w, req)
}

func unauthorizedHandler(w http.ResponseWriter, r *http.Request) {

	html, err := ioutil.ReadFile(filepath.Join(workingDirectory, "files/http-401-template.html"))
	if err != nil {
		html = []byte(`<html> <body> <span style="font-size: 9pt; color: #333;">401: Pas autoris&eacute; / Unauthorized</span> <h1>T'es qui, to&eacute;? // Who the heck are you?</h1> <p>Vous avez atteint un dossier qui n&eacute;cessite une identification avant d'atteindre le contenu. Vous devez remplir le formulaire qui vous avait &eacute;t&eacute; pr&eacute;sent&eacute; avant de voir le contenu.</p> <p>You have reached a folder or a file that requires authorization. You need to identify yourself before seeing that content.</p> </body> </html> `)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(html))
	return
}

func fileNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	html, err := ioutil.ReadFile(filepath.Join(workingDirectory, "files/http-404-template.html"))
	if err != nil {
		html = []byte(`<html> <body> <span style="font-size: 9pt; color: #333;">404: Fichier introuvable / File not found</span> <h1>Oops!</h1> <p>Tout comme la vie intelligente sur une autre plan&egrave;te, ce fichier ne semble pas exister... jusqu'&agrave; preuve du contraire!</p> <p>Just like intelligent life on another planet, this file doesn't exist... until proven otherwise!</p> </body> </html> `)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(html))
	return
}

/*****************************************************************************
								LOGGING
*****************************************************************************/
func accessLog(vHost string, r *http.Request, httpCode int) {
	//fmt.Printf("VHOST: %s\n", filepath.Base(vHost))
	f, err := os.OpenFile(path.Join(workingDirectory, "logs", fmt.Sprintf("%s.log", filepath.Base(vHost))), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0660)
	if err != nil {
		errorLog(LOG_ERROR, vHost, fmt.Sprintf("Error opening ", path.Join(workingDirectory, fmt.Sprintf("%s.log", vHost), err.Error())))
		return
	}
	defer f.Close()

	ip := r.RemoteAddr[:strings.LastIndex(r.RemoteAddr, ":")]
	t := time.Now().Format("2/Jan/2006:15:04:05 -0700")
	query := fmt.Sprintf("%s %s %s", r.Method, r.RequestURI, r.Proto)
	line := fmt.Sprintf("%s - - [%s] \"%s\" %d - %s\n", ip, t, query, httpCode, r.UserAgent())

	f.WriteString(line)
}

func errorLog(loglevel int, vHost string, text string) {
	if loggingEnabled == true && errLoggingLevel <= loglevel {
		t := time.Now().Format("2/Jan/2006:15:04:05 -0700")
		errorLevel := ""
		switch errLoggingLevel {
		case LOG_DEBUG:
			errorLevel = "debug"
		case LOG_INFO:
			errorLevel = "info"
		case LOG_ERROR:
			errorLevel = "error"
		case LOG_WARNING:
			errorLevel = "warning"
		}

		line := fmt.Sprintf("[%s] [%s] [] %s\n", t, errorLevel, text)

		switch errLoggingOutput {
		case ERR_LOG_FILE:
			f, err := os.OpenFile(path.Join(workingDirectory, "logs", fmt.Sprintf("%s.err", filepath.Base(vHost))), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0660)
			if err != nil {
				fmt.Println("Error opening ", path.Join(workingDirectory, fmt.Sprintf("%s.log", vHost), err.Error()))
				return
			}
			f.WriteString(line)
			return
		case ERR_LOG_STDOUT:
			fmt.Printf(line)
		case ERR_LOG_SILENT:
			//What did you expect? :-)
		}
	}
}

/*****************************************************************************
								AUTH
*****************************************************************************/

//shortcut function :)
func requireHttpAuth(w http.ResponseWriter, r *http.Request, realm string) {
	w.Header().Add("WWW-Authenticate", realm)
	unauthorizedHandler(w, r)
}

//Returns the first root-most folder for which the user would need to authenticate to.
//This server only requires authentication for the 'first' folder protected by .auth, 
//As it fit most basic needs anyway.
func needsAuth(vHostFolder string, path string) string {
	directories := ""
	dirs := strings.Split(path, "/")
	for _, v := range dirs { //v == "" ? => root vHostFolder (ne pas skipper)
		directories = filepath.Join(directories, v)
		if dirOk, _ := fileExists(filepath.Join(vHostFolder, directories)); dirOk {
			if fileOk, _ := fileExists(filepath.Join(vHostFolder, directories, ".auth")); fileOk {
				return filepath.Join(vHostFolder, directories, ".auth")
			}
		}
	}
	return ""
}

/*****************************************************************************
								MISC FUNCS
*****************************************************************************/

// true if file is an element in the hiddenFiles global variable
func fileIsDiscarded(file string) bool {
	f := filepath.Base(file)
	for _, v := range hiddenFiles {
		if v == f {
			return true
		}
	}
	return false
}

func fileIsDir(path string) (bool, error) {
	exists, err := fileExists(path)
	if exists != true || err != nil {
		return exists, err
	}

	file, err := os.Stat(path)
	return file.IsDir(), nil
}

func fileIsExecutable(path string) (bool, error) {
	exists, err := fileExists(path)
	if exists != true || err != nil {
		return exists, err
	}

	file, err := os.Stat(path)
	if file.IsDir() { //Ceci n'est pas un fichier.
		return false, nil
	}
	fileMode := file.Mode()
	if (fileMode & 0111) != 0 {
		return true, nil
	}
	return false, nil
}

func fileContainsLine(file string, text string) bool {
	fileContent, err := ioutil.ReadFile(file)
	if err != nil {
		return false //error reading file.
	}
	fileContentsString := string(fileContent)
	fileLines := strings.Split(fileContentsString, "\n")
	for _, v := range fileLines {
		if v == "" {
			continue
		}
		if v == text {
			return true
		}
	}

	return false
}

//From: http://stackoverflow.com/questions/10510691/how-to-check-whether-a-file-or-directory-denoted-by-a-path-exists-in-golang
func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

//Encodes a string as base64.
func toBase64(data string) string {
	var buf bytes.Buffer
	encoder := base64.NewEncoder(base64.StdEncoding, &buf)
	encoder.Write([]byte(data))
	encoder.Close()
	return buf.String()
}

//Decodes a base64 string to a... string.
func fromBase64(data string) string {
	buf := make([]byte, len(data)*2)
	r := base64.NewDecoder(base64.StdEncoding, strings.NewReader(data))
	b, _ := r.Read(buf)
	return string(buf[:b])
}
