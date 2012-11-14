//Packages de développement pour Rona L'Entrepot Sherbrooke
package rona

import (
	//"fmt"
	"os"
	"strings"
)

//Dans un array/slice ayant une VALEUR formatté tel que: [variable=valeur blah blah], 
//cette fonction nous separe la partie valeur en deux avec une séparation sur le premier
//caractère "=" trouvé. Ceci est particulièrement utile pour 'parser' les variables 
//d'environnement sur le système d'exploitation.
func SplitKVP(kvp []string) [][]string {
	rtn := make([][]string, len(kvp), len(kvp))
	for i, val := range kvp {
		tmp := strings.SplitN(val, "=", 2)
		rtn[i] = tmp
	}
	return rtn

}

//Dans un [][]string, trouver si une clé nommé 'key' (la variable) existe, et retourne
//la valeur correspondante ainsi qu'un boolean à savoir si on a trouvé la clé.
func GetKVP(kvp [][]string, key string) (string, bool) {
	for i, val := range kvp {
		if kvp[i][0] == key {
			if len(val) > 1 { //Il y a une valeur dans cette array
				return val[1], true
			} else { //PAS de valeur dans cette array.
				return "", true
			}
		}
	}
	return "", false
}

//Nous retourne un array d'array de string, contenant les variables d'environnement du OS, formattés comme: [[HOME /home/info] [SHELL /bin/bash]]
//
//Dans le cas d'une requête HTTP (via CGI), les variables du serveur web seront retournés (dans le même format).
func GetEnvironmentVariables() [][]string {
	return SplitKVP(os.Environ())
}

//Retrives the HTTP GET Arguments (?var=val&var2=val2)

//Nous retourne un array d'array de string contenant les arguments à l'url d'une requête HTTP GET. Tout ce qui apparait après le "?" dans l'URL sera considéré et rajouté dans le résultat.
//
//Ex:. http://localhost/?variable1=valeur1&variable2=valeur2 --> [[variable1 valeur1][variable2 valeur2]]
func GetHTTPArguments() [][]string {
	fullargs, exist := GetKVP(GetEnvironmentVariables(), "QUERY_STRING")
	if exist == true {
		kvp := strings.Split(fullargs, "&")
		kvpSlice := make([][]string, len(kvp), len(kvp))
		for i, k := range kvp {
			kvpSlice[i] = strings.Split(k, "=")
		}
		return kvpSlice
	}
	return make([][]string, 0)
}

//Retourne UNE argument d'un URL HTTP GET. S'il n'existe pas, "" sera retourné. Dans
//les deux cas, 'bool' indiquera si l'argument était existante dans l'URL.
func GetHTTPArgument(arg string) (string, bool) {
	args := GetHTTPArguments()
	val, exist := GetKVP(args, arg)
	if exist == false {
		return "", false
	}
	return val, true
}

//Retourne la valeur d'UNE variable d'environnement du système d'exploitation.
func GetEnv(variable string) string {
	vars := GetEnvironmentVariables()
	value, exists := GetKVP(vars, variable)
	if exists == false {
		return ""
	} else {
		return value
	}
	return value
}

//Vérifie si le script s'exécute via un shell (via console ou SSH). Retournera vrai si exécuté via shell, faux sinon.
func IsShell() bool {
	environ := GetEnvironmentVariables()
	_, exist := GetKVP(environ, "HOME")
	return exist
}

//Vérifie si le script s'exécute via serveur web (via CGI). Retournera vrai si exécuté via un serveur web, faux sinon.
func IsHTTP() bool {
	environ := GetEnvironmentVariables()
	_, exist := GetKVP(environ, "REQUEST_URI")
	return exist
}
