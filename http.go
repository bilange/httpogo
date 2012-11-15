package main

import (
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
	case strings.HasSuffix(r.URL.Path, ".md"):
		markdownHandler(w, r, fileAbsolute)
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
			http.ServeFile(w, r, filepath.Join(vHostFolder, r.URL.Path))
			return
		} else {
			http.ServeFile(w, r, filepath.Join(vHostFolder, r.URL.Path))
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

func markdownHandler(w http.ResponseWriter, req *http.Request, file string) {
	header := `
<!DOCTYPE html><html><head><meta charset="UTF-8"><style>html { font-size: 100%; overflow-y: scroll; -webkit-text-size-adjust: 100%; -ms-text-size-adjust: 100%; }

body{
  color:#444;
  font-family:Georgia, Palatino, 'Palatino Linotype', Times,
              'Times New Roman', serif,
              "Hiragino Sans GB", "STXihei", "微软雅黑";
  font-size:12px;
  line-height:1.5em;
  background:#fefefe;
  width: 45em;
  margin: 10px auto;
  padding: 1em;
  outline: 1300px solid #FAFAFA;
}

a{ color: #0645ad; text-decoration:none;}
a:visited{ color: #0b0080; }
a:hover{ color: #06e; }
a:active{ color:#faa700; }
a:focus{ outline: thin dotted; }
a:hover, a:active{ outline: 0; }

span.backtick {
  border:1px solid #EAEAEA;
  border-radius:3px;
  background:#F8F8F8;
  padding:0 3px 0 3px;
}

::-moz-selection{background:rgba(255,255,0,0.3);color:#000}
::selection{background:rgba(255,255,0,0.3);color:#000}

a::-moz-selection{background:rgba(255,255,0,0.3);color:#0645ad}
a::selection{background:rgba(255,255,0,0.3);color:#0645ad}

p{
margin:1em 0;
}

img{
max-width:100%;
}

h1,h2,h3,h4,h5,h6{
font-weight:normal;
color:#111;
line-height:1em;
}
h4,h5,h6{ font-weight: bold; }
h1{ font-size:2.5em; }
h2{ font-size:2em; border-bottom:1px solid silver; padding-bottom: 5px; }
h3{ font-size:1.5em; }
h4{ font-size:1.2em; }
h5{ font-size:1em; }
h6{ font-size:0.9em; }

blockquote{
color:#666666;
margin:0;
padding-left: 3em;
border-left: 0.5em #EEE solid;
}
hr { display: block; height: 2px; border: 0; border-top: 1px solid #aaa;border-bottom: 1px solid #eee; margin: 1em 0; padding: 0; }


pre , code, kbd, samp { 
  color: #000; 
  font-family: monospace; 
  font-size: 0.88em; 
  border-radius:3px;
  background-color: #F8F8F8;
  border: 1px solid #CCC; 
}
pre { white-space: pre; white-space: pre-wrap; word-wrap: break-word; padding: 5px;}
pre code { border: 0px !important; }
code { padding: 0 3px 0 3px; }

b, strong { font-weight: bold; }

dfn { font-style: italic; }

ins { background: #ff9; color: #000; text-decoration: none; }

mark { background: #ff0; color: #000; font-style: italic; font-weight: bold; }

sub, sup { font-size: 75%; line-height: 0; position: relative; vertical-align: baseline; }
sup { top: -0.5em; }
sub { bottom: -0.25em; }

ul, ol { margin: 1em 0; padding: 0 0 0 2em; }
li p:last-child { margin:0 }
dd { margin: 0 0 0 2em; }

img { border: 0; -ms-interpolation-mode: bicubic; vertical-align: middle; }

table { border-collapse: collapse; border-spacing: 0; }
td { vertical-align: top; }

@media only screen and (min-width: 480px) {
body{font-size:14px;}
}

@media only screen and (min-width: 768px) {
body{font-size:16px;}
}

@media print {
  * { background: transparent !important; color: black !important; filter:none !important; -ms-filter: none !important; }
  body{font-size:12pt; max-width:100%;}
  a, a:visited { text-decoration: underline; }
  hr { height: 1px; border:0; border-bottom:1px solid black; }
  a[href]:after { content: " (" attr(href) ")"; }
  abbr[title]:after { content: " (" attr(title) ")"; }
  .ir a:after, a[href^="javascript:"]:after, a[href^="#"]:after { content: ""; }
  pre, blockquote { border: 1px solid #999; padding-right: 1em; page-break-inside: avoid; }
  tr, img { page-break-inside: avoid; }
  img { max-width: 100% !important; }
  @page :left { margin: 15mm 20mm 15mm 10mm; }
  @page :right { margin: 15mm 10mm 15mm 20mm; }
  p, h2, h3 { orphans: 3; widows: 3; }
  h2, h3 { page-break-after: avoid; }
}</style>
`
	footer := "</html>"

	md, err := ioutil.ReadFile(file)
	if err == nil {
		output := blackfriday.MarkdownCommon(md)
		w.Write([]byte(header))
		w.Write(output)
		w.Write([]byte(footer))
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
