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
var loggingEnabled bool = false          // Activate logging?

var errLoggingOutput int = ERR_LOG_STDOUT //	Where should we print out errors/debug?
var errLoggingLevel int = LOG_INFO        // Minimal logging

var hiddenFiles []string = []string{ // Files we NEVER want shown
	".auth",
	".bin",
	".cgi",
}

//Lance le serveur web.
//commandline parameters: 
// -port == TCP port sur lequel le serveur ecoutera.
// -root == dossier racine qui sera servi aux clients HTTP. ATTENTION, le dossier racine doit contenir
//          un dossier au nom du domaine demandé par l'usager. Par exemple, si on veut que le serveur réponde
//          sous www.ronasherbrooke.com, on doit créer un sous-dossier "www.ronasherbrooke.com" sous le dossier
//          root.
// -webdir == Sous-dossier qui servira de dossier HTML public par defaut, dans le cas ou aucun sous-dossier 
//			  match en tant que virtual-host.
func main() {
	absoluteWd, _ := os.Getwd() //par defaut, le dossier contenant l'executable servira de workingDirectory.

	parsedPort := flag.Int("port", 80, "Port TCP sur lequel le serveur va ecouter")
	parsedWorkingDirectory := flag.String("root", absoluteWd, "Dossier de travail du serveur web (bins, scripts, et v-host folders)")
	parsedDefaultVHost := flag.String("webdir", "public_html", "V-Host par default, si aucune requete match avec un sous-dossier de --root ")
	parsedLog := flag.Bool("log", false, "Doit-on loguer les requetes a l'ecran?")
	parsedLogLevel := flag.String("loglevel", "error", "Quel niveau de verbosite doit-on loguer? DEBUG|INFO|WARNING|ERROR")
	parsedErrLog := flag.String("errorto", "stdout", "Ou doit-on loguer les erreurs? SILENT|STDOUT|FILE")

	flag.Parse()

	port = *parsedPort
	workingDirectory = *parsedWorkingDirectory
	defaultVHost = *parsedDefaultVHost
	loggingEnabled = *parsedLog

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

//requestHandler se charge de la connection, cherche si un v-host en tant que
//repertoire existe, et s'occupe de dispatcher le fichier demande a l'usager.
//Dans le cas d'un fichier PHP ou binaire (ou script shell), la controle est 
//renvoye au fichier externe en l'executant convenablement (CGI).
//BUG: En passant dans un dossier qui est un symlink, toutes les requetes de
//dossiers (qui generent un listing de fichiers) donneront des mauvais URLs.
//Ex: http:host/folder/, si 'folder' n'a pas d'index.html et que ce dossier
//est un lien symbolique vers un autre dossier, ceci donnera des urls du genre
//http:host/filename.ext EN OMETTANT le dossier symlink. Ceci dit, faire une
//requete ou le fichier serait suppose d'exister en passant dans un symlink
//fonctionne correctement.
func requestHandler(w http.ResponseWriter, r *http.Request) {

	pwd := workingDirectory
	hostSplit := strings.Split(r.Host, ":")
	host := hostSplit[0]

	vHostFolder := path.Join(pwd, host)
	vHostDirExists, _ := fileIsDir(vHostFolder)
	if vHostDirExists == true {
		pwd = vHostFolder
	} else {
		pwd = path.Join(pwd, defaultVHost) //Fallback Default. 
		vHostFolder = pwd
	}

	errorLog(LOG_DEBUG, host, fmt.Sprintf("vHost Folder: %s", vHostFolder))

	fileAbsolute := filepath.Join(pwd, r.URL.Path)

	errorLog(LOG_DEBUG, host, fmt.Sprintf("%s:%s -> %s ", host, r.URL.Path, fileAbsolute))

	if fileIsDiscarded(r.URL.Path) {
		errorLog(LOG_WARNING, host, fmt.Sprintf("Filename '%s' is returned to the client as '404 not found' due to being used internally by the server. If this is a legitimate file, change the file name to something else. ", fileAbsolute))
		accessLog(vHostFolder, r, 404)
		fileNotFoundHandler(w, r)
		return
	}

	//Pour les fichiers non-existants 404.
	fexists, _ := fileExists(fileAbsolute)
	if fexists == false && !strings.HasSuffix(r.URL.Path, ".md.txt") {
		accessLog(vHostFolder, r, 404)
		fileNotFoundHandler(w, r)
		return
	}

	authFile := needsAuth(vHostFolder, r.URL.Path)
	if authFile != "" {
		//L'usager doit se loguer pour voir le contenu de ce dossier. Une boite user/password sera affiche a l'ecran.
		//l'usager restera logue jusqu'a temps que l'usager ferme le browser ou consulte un autre dossier requierant un 
		//autre login/password pour le meme domaine.
		//Voir : http://en.wikipedia.org/wiki/Basic_access_authentication
		//
		// Le fichier .auth contient, un par ligne : 
		//     USERNAME:PASSWORD
		// (Attention le password est gardé en texte clair!!!!!)
		userAuth := r.Header["Authorization"]
		if userAuth == nil { //L'usager n'a pas inscrit de user/pass pour un dossier en requierant un.
			requireHttpAuth(w, r, fmt.Sprintf("Basic realm=\"%s\"", strings.Replace(filepath.Dir(authFile), vHostFolder, "", -1)))
			accessLog(vHostFolder, r, 401)
			return
		} else {
			userAuthParts := strings.Split(userAuth[0], " ")
			if len(userAuthParts) == 2 {
				userAuthEncoded := userAuthParts[1]
				userAuthDecoded := fromBase64(userAuthEncoded)
				if !fileContainsLine(authFile, userAuthDecoded) { //Le fichier ne contient pas de user/password specifie par l'usager.
					requireHttpAuth(w, r, fmt.Sprintf("Basic realm=\"%s\"", strings.Replace(filepath.Dir(authFile), vHostFolder, "", -1)))
					accessLog(vHostFolder, r, 401)
					return
				}
				// L'usager est authentifie, on peut laisser passer a partir de ce point.
			} else { //Mauvaise requete HTTP pour l'auth
				requireHttpAuth(w, r, fmt.Sprintf("Basic realm=\"%s\"", strings.Replace(filepath.Dir(authFile), vHostFolder, "", -1)))
				accessLog(vHostFolder, r, 401)
				return
			}
		}
	}

	if authFile == "" {
		errorLog(LOG_DEBUG, host, "No auth found, carrying on.")
	}

	phpActuallyBinary := (r.URL.Path == "/backend.php" || r.URL.Path == "/cron.php") //hard-coded exceptions
	if strings.HasSuffix(r.URL.Path, ".php") == true && (!phpActuallyBinary) {       //Fichier PHP. Ceci requiert php-cgi.
		phpHandler(w, r, r.URL.Path)
		return
	}

	fdir, _ := fileIsDir(fileAbsolute) //Le URL demande est en fait un dossier
	if fdir == true {
		fileAbsolute += string(os.PathSeparator)
	}

	mimeType := mime.TypeByExtension(filepath.Ext(fileAbsolute))
	fexecutable, _ := fileIsExecutable(fileAbsolute)

	switch {
	//case strings.HasSuffix(r.URL.Path, ".auth"): //on refuse .auth pour raisons de securite.
	//fileNotFoundHandler(w, r) //SECURE: URL.Path n'a que le fichier, sans
	//return                    // ?param ou #anchor dans l'url.
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
			// Si par contre un fichier "index" existe, servir ce fichier plutot.
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
			// Repertoire ouvert sans index a presenter. On affiche les fichiers ("open directory")
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
		response.WriteString(fmt.Sprintf("Erreur d'ouverture du dossier %s: %s\n", directory, err.Error()))
		return
	}

	// On affiche d'abord les dossiers, ensuite les fichiers.
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

	// On s'occupe du template et on affiche le tout au client.
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

//phpHandler se charge des scripts PHP, pour backward-compatibility.
//Attention, php-cgi est necessaire pour ce setup dans le meme dossier que le
//serveur http.
func phpHandler(w http.ResponseWriter, req *http.Request, script string) {
	//pwd, _ := os.Getwd()
	pwd := workingDirectory

	hostSplit := strings.Split(req.Host, ":")
	host := hostSplit[0]

	vHostFolder := path.Join(pwd, host)
	//vHostDirExists, _ := fileIsDir(vHostFolder)
	//if vHostDirExists == true {
	if ok, _ := fileIsDir(vHostFolder); ok {
		pwd = vHostFolder
	} else {
		pwd = path.Join(pwd, defaultVHost)
	}

	cgiHandler := cgi.Handler{
		//Path: path.Join(pwd, "../php-cgi"),
		Path: path.Join(workingDirectory, "files/php-cgi"),
		Dir:  pwd,
		Root: pwd,
		Args: []string{req.URL.Path},
		Env: []string{
			"REDIRECT_STATUS=200",
			//"SCRIPT_FILENAME=" + path.Join(pwd, req.URL.Path),
			//"SCRIPT_NAME=" + path.Join(pwd, req.URL.Path),
			"SCRIPT_FILENAME=" + path.Join(pwd, script),
			"SCRIPT_NAME=" + path.Join(pwd, script),
		},
	}
	//errorLog(LOG_DEBUG, fmt.Sprintf("CGI Handler: %#v", cgiHandler))
	cgiHandler.ServeHTTP(w, req)
}

//executableHandler se charge des fichiers executables, tel des programmes go 
//compiles, des shell scripts et autres programmes dont on n'a pas le controle.
//L'usager et l'executable est entierement responsable du contenu, on ne fait 
//que le facteur.
func executableHandler(w http.ResponseWriter, req *http.Request, bin string) {
	//pwd, _ := os.Getwd()
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
			//rien??
		}
	}
}

/*****************************************************************************
								AUTH
*****************************************************************************/
func requireHttpAuth(w http.ResponseWriter, r *http.Request, realm string) {
	w.Header().Add("WWW-Authenticate", realm)
	//w.WriteHeader(http.StatusUnauthorized)
	unauthorizedHandler(w, r)
}

func needsAuth(vHostFolder string, path string) string {
	directories := ""
	dirs := strings.Split(path, "/")
	//fmt.Printf("len: %d %#v\n", len(dirs), dirs)
	for _, v := range dirs { //v == "" ? => root vHostFolder (ne pas skipper)
		directories = filepath.Join(directories, v)
		//println("Walking ", filepath.Join(vHostFolder, directories))
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
		//fmt.Printf("%#v versus %#v\n", v, text)
		if v == text {
			return true
		}
	}

	return false
}

//De: http://stackoverflow.com/questions/10510691/how-to-check-whether-a-file-or-directory-denoted-by-a-path-exists-in-golang
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

//Encode un string en tant que base64.
func toBase64(data string) string {
	var buf bytes.Buffer
	encoder := base64.NewEncoder(base64.StdEncoding, &buf)
	encoder.Write([]byte(data))
	encoder.Close()
	return buf.String()
}

//Decode un string base64.
func fromBase64(data string) string {
	buf := make([]byte, len(data)*2)
	r := base64.NewDecoder(base64.StdEncoding, strings.NewReader(data))
	b, _ := r.Read(buf)
	return string(buf[:b])
}
