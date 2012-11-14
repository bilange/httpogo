package main

import (
	"flag"
	"fmt"
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
	phpRegexp, _ := regexp.Compile(".*\\.php")

	//pwd, _ := os.Getwd()
	pwd := wwwRoot

	hostSplit := strings.Split(r.Host, ":")
	host := hostSplit[0]

	vHostFolder := path.Join(pwd, host)
	vHostDirExists, _ := rona.FileIsDir(vHostFolder)
	if vHostDirExists == true {
		pwd = vHostFolder
	} else {
		pwd = path.Join(pwd, "10.6.41.10")
	}

	fileAbsolute := filepath.Join(pwd, r.URL.Path)

	if r.URL.Path == "/favicon.ico" { //Logging flood, on skip.
		http.NotFound(w, r)
		return
	}

	//Pour les fichiers non-existants 404.
	fexists, _ := rona.FileExists(fileAbsolute)
	if fexists == false {
		fileNotFoundHandler(w, r)
		return
	}

	phpActuallyBinary := (r.URL.Path == "/backend.php" || r.URL.Path == "/cron.php") //hard-coded exceptions
	if phpRegexp.MatchString(r.URL.Path) == true && (!phpActuallyBinary) {           //Fichier PHP. Ceci requiert php-cgi.
		phpHandler(w, r)
		return
	}

	fdir, _ := rona.FileIsDir(fileAbsolute) //Le URL demande est en fait un dossier
	if fdir == true {
		fileAbsolute += string(os.PathSeparator)
	}

	mimeType := mime.TypeByExtension(filepath.Ext(fileAbsolute))
	fexecutable, _ := rona.FileIsExecutable(fileAbsolute)

	switch {
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
		//http.ServeFile(w, r, "./"+r.URL.Path)
		http.ServeFile(w, r, filepath.Join(vHostFolder, r.URL.Path))
	default:
		if fdir == true {
			//http.ServeFile(w, r, "./"+r.URL.Path)
			http.ServeFile(w, r, filepath.Join(vHostFolder, r.URL.Path))
			return
		}
	}
}

var port int = 80
var wwwRoot string = "/var/www"

//Lance le serveur web.
//commandline parameters: 
// -port == TCP port sur lequel le serveur ecoutera.
// -root == dossier racine qui sera servi aux clients HTTP. ATTENTION, le dossier racine doit contenir
//          un dossier au nom du domaine demandé par l'usager. Par exemple, si on veut que le serveur réponde
//          sous www.ronasherbrooke.com, on doit créer un sous-dossier "www.ronasherbrooke.com" sous le dossier
//          root.
func main() {
	absoluteWd, _ := os.Getwd() //par defaut, le dossier contenant l'executable servira de wwwRoot.

	parsedPort := flag.Int("port", 80, "Port TCP sur lequel le serveur va ecouter")
	parsedWWWRoot := flag.String("root", absoluteWd, "Chemin de base vers lequel le serveur web va fournir les fichiers")

	flag.Parse()

	port = *parsedPort
	wwwRoot = *parsedWWWRoot

	http.HandleFunc("/", requestHandler)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		println("Erreur de demarrage du serveur web: ", err.Error())
	}
	return
}

//phpHandler se charge des scripts PHP, pour backward-compatibility.
//Attention, php-cgi est necessaire pour ce setup dans le meme dossier que le
//serveur http.
func phpHandler(w http.ResponseWriter, req *http.Request) {
	//pwd, _ := os.Getwd()
	pwd := wwwRoot

	hostSplit := strings.Split(req.Host, ":")
	host := hostSplit[0]

	vHostFolder := path.Join(pwd, host)
	vHostDirExists, _ := rona.FileIsDir(vHostFolder)
	if vHostDirExists == true {
		pwd = vHostFolder
	} else {
		pwd = path.Join(pwd, "10.6.41.10") //Default host/folder. TODO variable?
	}

	cgiHandler := cgi.Handler{
		Path: path.Join(pwd, "../php-cgi"),
		Dir:  pwd,
		Root: pwd,
		Args: []string{req.URL.Path},
		Env: []string{
			"REDIRECT_STATUS=200",
			"SCRIPT_FILENAME=" + path.Join(pwd, req.URL.Path),
			"SCRIPT_NAME=" + path.Join(pwd, req.URL.Path),
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
	pwd := wwwRoot

	hostSplit := strings.Split(req.Host, ":")
	host := hostSplit[0]

	vHostFolder := path.Join(pwd, host)
	vHostDirExists, _ := rona.FileIsDir(vHostFolder)
	if vHostDirExists == true {
		pwd = vHostFolder
	} else {
		pwd = path.Join(pwd, "10.6.41.10")
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
	w.WriteHeader(http.StatusNotFound)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
	return
}
