package rona

import (
	"fmt"
	"github.com/ziutek/mymysql/mysql"
	_ "github.com/ziutek/mymysql/native"
	"os"
	"strings"
	"time"
)

//Cette variable contient le contenu principal (le code HTML ou XML,en général) qui sera renvoyé au client.
//À noter que le programme a été monté en fonction d'accumuler les en-têtes et contenus dans un string pour être éventuellement poussé au client (le tout manuellement), dans le but de pouvoir se créer une porte de sortie, tel que changer d'idée et renvoyer au client un XML d'erreur si une requête SQL tombe en échec.
var HttpContent string

//Cette variable contient toutes les en-tête HTTP qui seront renvoyés au client. Ne PAS rajouter l'en-tete "Content-Type" ici, utiliser HttpContentType.
//À noter que le programme a été monté en fonction d'accumuler les en-têtes et contenus dans un string pour être éventuellement poussé au client (le tout manuellement), dans le but de pouvoir se créer une porte de sortie, tel que changer d'idée et renvoyer au client un XML d'erreur si une requête SQL tombe en échec.
var HttpHeader string

//Cette variable contient SEULEMENT l'en-tête HTTP "Content-Type" qui sera renvoyé au client. Cette variable sera rajouté à la variable HttpHeader.
//À noter que le programme a été monté en fonction d'accumuler les en-têtes et contenus dans un string pour être éventuellement poussé au client (le tout manuellement), dans le but de pouvoir se créer une porte de sortie, tel que changer d'idée et renvoyer au client un XML d'erreur si une requête SQL tombe en échec.
var HttpContentType string

//Definit l'en-tête HTTP "Content-Type", en ajoutant que l'encodage sera en UTF8.
func HttpSetContentTypeUTF(str string) {
	HttpContentType = str + "; charset=utf-8"
}

//Definit l'en-tête HTTP "Content-Type".
func HttpSetContentType(str string) {
	HttpContentType = str
}

//Rajoute 'value' comme réponse déjà gardé à date pour le contenu HTTP.
func HttpWriteResponse(value string) {
	HttpContent = HttpContent + value
}

//Flush la réponse qu'on avait gardé jusqu'à présent et utilise 'value' comme nouveau contenu HTTP.
func HttpSetResponse(value string) {
	HttpContent = value
}

//"Flush" ce qu'on a sauvegardé comme en-tetes jusqu'ici, et la remplace par ce qu'on a d'inscrit dans value.
func HttpSetHeader(value string) {
	HttpHeader = value
}

//Rajoute aux en-têtes sauvegardés ce qu'on a  d'inscrit dans value.
func HttpWriteHeader(value string) {
	HttpHeader = HttpHeader + value
}

//Renvoit au client le contenu des valeurs contenus dans HttpContentType, HttpHeader et HttpContent, et complète la requête HTTP.
func FlushHttp(errorCode int) {
	if HttpContentType == "" {
		HttpContentType = "text/html"
	}

	//fmt.Printf("\n\n.\n\nHeader: %s\n\n.\n\nContent-Type would be %s\n\n.\n\n%s", HttpHeader, HttpContentType, HttpContent)

	fmt.Printf("Content-Type: %s\n%s\n%s", HttpContentType, HttpHeader, HttpContent)

	os.Exit(errorCode)
}

//Se connecte au serveur MySQL, fait un query, et retourne le résultat dans un array.
func GetSQL(sql string) (error, []mysql.Row) {
	db := mysql.New("tcp", "", "127.0.0.1:3306", "gesadm", "mdaseg", "ogc")

	err := db.Connect()
	if err != nil {
		HttpWriteResponse(fmt.Sprint("Paniced on connect: ", err))
		return err, nil
	}
	//} else {
	//fmt.Print("Connected!")
	//db.Close()
	//}

	rows, _, err := db.Query(sql)
	if err != nil {
		HttpWriteResponse(fmt.Sprint("Paniced on query: ", err))
		return err, nil
	}
	//fmt.Print(sql,": ",rows)

	for _, row := range rows {
		for _, col := range row {
			if col != nil {
				//Do Something
			}
		}

		//val1 := row[1].([]byte)
		//val1 := row[0]
		//fmt.Print("Rona: ", row.Str(0), "\n")
		//os.Stdout.Write(val1)
	}
	return nil, rows

	//fmt.Print("rows: ",rows, "\n\n")
	//fmt.Print("res: ",res, "\n\n")
	//for _, row := range rows {
}

//Securise le user input pour les queries SQL. 
func SQLSafeEscape(val string) string {
	//liste provenant de: https://www.owasp.org/index.php/SQL_Injection_Prevention_Cheat_Sheet
	escapables := [][]string{
		{"\u005c", "\\\u005c"}, //\\
		{"\u0000", "\\\u0000"}, //NUL
		{"\u0008", "\\\u0008"}, //backspace
		{"\u0009", "\\\u0009"}, //TAB
		{"\u000a", "\\\u000a"}, //Line feed
		{"\u000d", "\\\u000d"}, //Carriage return
		{"\u001a", "\\\u001a"}, //\z (sub?)
		{"\u0022", "\\\u0022"}, //"
		{"\u0025", "\\\u0025"}, //%
		{"\u0027", "\\\u0027"}, //'
		{"\u005f", "\\\u005f"}, //_
	}
	for _, v := range escapables {
		val = strings.Replace(val, v[0], v[1], -1)
	}
	return val

}

func FileIsDir(path string) (bool, error) {
	exists, err := FileExists(path)
	if exists != true || err != nil {
		return exists, err
	}

	file, err := os.Stat(path)
	return file.IsDir(), nil
}

func FileIsExecutable(path string) (bool, error) {
	exists, err := FileExists(path)
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

//De: http://stackoverflow.com/questions/10510691/how-to-check-whether-a-file-or-directory-denoted-by-a-path-exists-in-golang
func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func Timestamp() string {
	return time.Now().Format(time.ANSIC)
}
