package rona

import (
	"strings"
)


//Vérifie si, pour une slice data, nous avons une clé 'key'. caseSensitive indique si
//on doit faire fi de la casse dans le nom de la clé. 
//Retourne true si trouvé, false sinon
func HasKeyCaseSensitive(data []string, key string, caseSensitive bool) bool {
	for _, k := range data {
		if caseSensitive == true {
			if strings.ToLower(k) == strings.ToLower(key) { return true }
		} else {
			if k == key { return true }
		}
	}
	return false
}


//Vérifie si, pour une slice data, nous avons une clé 'key'.
//Retourne true si trouvé, false sinon
func HasKey(data []string, key string) bool {
	return HasKeyCaseSensitive(data, key, false)
}
