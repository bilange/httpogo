/*  ajout bidon

ATTENTION: a *chaque* fois qu'on compile le programme et qu'on ecrase le
fichier executable d'origine où le serveur sera exécuté, on DOIT ABSOLUMENT
faire la commande suivante en tant que root:

	setcap cap_net_bind_service=+ep /chemin/vers/fichier/compilé

Ceci nous donne la possibilité d'ouvrir un port TCP < 1024 en tant qu'un
usager UNIX non-root.

*/

package main

import (
	"bytes"
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
	"regexp"
	"rona"
	"strings"
)

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
	// TODO: Garder dans un array global une liste de fichiers 'dangereux', tel
	// .auth .

	phpRegexp, _ := regexp.Compile(".*\\.php")

	pwd := workingDirectory
	hostSplit := strings.Split(r.Host, ":")
	host := hostSplit[0]

	vHostFolder := path.Join(pwd, host)
	vHostDirExists, _ := rona.FileIsDir(vHostFolder)
	if vHostDirExists == true {
		pwd = vHostFolder
	} else {
		pwd = path.Join(pwd, defaultVHost) //Fallback Default. 
	}

	fileAbsolute := filepath.Join(pwd, r.URL.Path)

	if r.URL.Path == "/favicon.ico" { //Logging flood, on skip.
		http.NotFound(w, r)
		return
	}

	if logRequest == true {
		fmt.Printf("%s:%s -> %s \n", host, r.URL.Path, fileAbsolute)
	}

	//Pour les fichiers non-existants 404.
	fexists, _ := rona.FileExists(fileAbsolute)
	if fexists == false && !strings.HasSuffix(r.URL.Path, ".md.txt") {
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
			requireHttpAuth(w, fmt.Sprintf("Basic realm=\"%s\"", strings.Replace(filepath.Dir(authFile), vHostFolder, "", -1)))
			return
		} else {
			userAuthParts := strings.Split(userAuth[0], " ")
			if len(userAuthParts) == 2 {
				userAuthEncoded := userAuthParts[1]
				userAuthDecoded := rona.FromBase64(userAuthEncoded)
				if !fileContainsLine(authFile, userAuthDecoded) { //Le fichier ne contient pas de user/password specifie par l'usager.
					requireHttpAuth(w, fmt.Sprintf("Basic realm=\"%s\"", strings.Replace(filepath.Dir(authFile), vHostFolder, "", -1)))
					return
					//w.Header().Add("WWW-Authenticate", fmt.Sprintf("Basic realm=\"%s\"", strings.Replace(filepath.Dir(authFile), vHostFolder, "", -1)))
					//w.WriteHeader(http.StatusUnauthorized)
				}
				// L'usager est authentifie, on peut laisser passer a partir de ce point.
			} else { //Mauvaise requete HTTP pour l'auth
				requireHttpAuth(w, fmt.Sprintf("Basic realm=\"%s\"", strings.Replace(filepath.Dir(authFile), vHostFolder, "", -1)))
				return
			}
		}
	}

	phpActuallyBinary := (r.URL.Path == "/backend.php" || r.URL.Path == "/cron.php") //hard-coded exceptions
	if phpRegexp.MatchString(r.URL.Path) == true && (!phpActuallyBinary) {           //Fichier PHP. Ceci requiert php-cgi.
		phpHandler(w, r, r.URL.Path)
		return
	}

	fdir, _ := rona.FileIsDir(fileAbsolute) //Le URL demande est en fait un dossier
	if fdir == true {
		fileAbsolute += string(os.PathSeparator)
	}

	mimeType := mime.TypeByExtension(filepath.Ext(fileAbsolute))
	fexecutable, _ := rona.FileIsExecutable(fileAbsolute)

	switch {
	case strings.HasSuffix(r.URL.Path, ".auth"): //on refuse .auth pour raisons de securite.
		fileNotFoundHandler(w, r) //SECURE: URL.Path n'a que le fichier, sans
		return                    // ?param ou #anchor dans l'url.
	case strings.HasSuffix(r.URL.Path, ".md"):
		markdownHandler(w, r, fileAbsolute, false)
		return
	case strings.HasSuffix(r.URL.Path, ".md.txt"):
		markdownHandler(w, r, fileAbsolute, true)
		return
	case mimeType == "application/octet-stream",
		mimeType == "" && fexecutable == true,
		strings.HasPrefix(mimeType, "text/x-sh"), phpActuallyBinary:
		executableHandler(w, r)
		return
	case strings.HasPrefix(mimeType, "image"),
		strings.HasPrefix(mimeType, "text"),
		strings.HasPrefix(mimeType, "video"), strings.HasPrefix(mimeType, "audio"),
		strings.HasPrefix(mimeType, "music"),
		mimeType == "application/xml", mimeType == "application/javascript":

		http.ServeFile(w, r, filepath.Join(vHostFolder, r.URL.Path))
	default:
		if fdir == true {
			// Si par contre index.html / index.php existe dans le dossier, servir ce fichier
			// plutot.
			if ok, _ := rona.FileExists(filepath.Join(fileAbsolute, "index.html")); ok {
				http.ServeFile(w, r, filepath.Join(fileAbsolute, "index.html"))
				return
			}
			if ok, _ := rona.FileExists(filepath.Join(fileAbsolute, "index.php")); ok {
				phpHandler(w, r, filepath.Join(fileAbsolute, "index.php"))
				return
			}
			// Repertoire ouvert sans index a presenter. On affiche les fichiers ("open directory")
			directoryHandler(w, r, fileAbsolute)
			return
		} else {
			http.ServeFile(w, r, filepath.Join(vHostFolder, r.URL.Path))
		}
	}
}

var port int = 80
var workingDirectory string = "/var/www"
var defaultVHost string = "public_html"
var logRequest bool = false

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

	flag.Parse()

	port = *parsedPort
	workingDirectory = *parsedWorkingDirectory
	defaultVHost = *parsedDefaultVHost
	logRequest = *parsedLog

	http.HandleFunc("/", requestHandler)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		fmt.Printf("Erreur: %s\n", err.Error())
	}
	return
}

func needsAuth(vHostFolder string, path string) string {

	directories := ""
	dirs := strings.Split(path, "/")
	//fmt.Printf("len: %d %#v\n", len(dirs), dirs)
	for _, v := range dirs { //v == "" ? => root vHostFolder (ne pas skipper)
		directories = filepath.Join(directories, v)
		//println("Walking ", filepath.Join(vHostFolder, directories))
		if dirOk, _ := rona.FileExists(filepath.Join(vHostFolder, directories)); dirOk {
			if fileOk, _ := rona.FileExists(filepath.Join(vHostFolder, directories, ".auth")); fileOk {
				return filepath.Join(vHostFolder, directories, ".auth")
			}
		}
	}

	return ""
}

func directoryHandler(w http.ResponseWriter, req *http.Request, directory string) {
	w.Header().Add("Content-type", "text/html")

	var response bytes.Buffer

	template, err := ioutil.ReadFile(filepath.Join(workingDirectory, "dirlist-template.html"))
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
		template, err := ioutil.ReadFile(filepath.Join(workingDirectory, "markdown-template.html"))
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
	vHostDirExists, _ := rona.FileIsDir(vHostFolder)
	if vHostDirExists == true {
		pwd = vHostFolder
	} else {
		pwd = path.Join(pwd, defaultVHost)
	}

	cgiHandler := cgi.Handler{
		Path: path.Join(pwd, "../php-cgi"),
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
	cgiHandler.ServeHTTP(w, req)
}

//executableHandler se charge des fichiers executables, tel des programmes go 
//compiles, des shell scripts et autres programmes dont on n'a pas le controle.
//L'usager et l'executable est entierement responsable du contenu, on ne fait 
//que le facteur.
func executableHandler(w http.ResponseWriter, req *http.Request) {
	//pwd, _ := os.Getwd()
	pwd := workingDirectory

	hostSplit := strings.Split(req.Host, ":")
	host := hostSplit[0]

	vHostFolder := path.Join(pwd, host)
	vHostDirExists, _ := rona.FileIsDir(vHostFolder)
	if vHostDirExists == true {
		pwd = vHostFolder
	} else {
		pwd = path.Join(pwd, defaultVHost)
	}
	cgiHandler := cgi.Handler{
		Path: path.Join(pwd, req.URL.Path),
		Dir:  pwd,
		Root: pwd,
		//Args: []string{file},
		//Env:  []string{"SCRIPT_FILENAME=" + file},
	}
	cgiHandler.ServeHTTP(w, req)
}

func fileNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	html := `<html>
	<body>
	<span style="font-size: 9pt; color: #333;">404: Fichier introuvable / File not found</span>
	<h1>Oops!</h1>
	<p>Tout comme la vie intelligente sur une autre plan&egrave;te, ce fichier ne semble pas exister... jusqu'&agrave; preuve du contraire!</p>
	<p>Just like intelligent life on another planet, this file doesn't exist... until proven otherwise!</p>
	
	</body>
</html>
	`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(html))
	return
}

func requireHttpAuth(w http.ResponseWriter, realm string) {
	w.Header().Add("WWW-Authenticate", realm)
	w.WriteHeader(http.StatusUnauthorized)
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
