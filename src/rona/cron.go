/*
* Emulation cron.php
 */

package rona

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type Rapport struct {
	Titre      string   //Titre (e-mail, nom de la pièce jointe) du rapport
	Parametres string   //Parametres de ligne de commande sur OGC-Plus.
	EMail      []string //Liste d'e-mails a qui envoyer ce rapport lorsque extrait. 
	//Suppose que nous n'imprimons pas ce rapport pour envoyer
	//un e-mail.
}

var logBuffer string
var logToConsole bool = true

func log(line ...string) {
	for _, l := range line {
		logBuffer += fmt.Sprintf("%s %s\n", Timestamp(), l)
		if logToConsole == true {
			fmt.Printf("%s %s\n", Timestamp(), l)
		}
	}
}

//Retourne une date en string au format A-M-J. Pour rapports SysACE.
//func ogcDate(t time.Time) string {
//return t.Format("2006-01-02")
//}

//Retourne une date en string au format A-M-J. Pour rapports SysACE.
func ogcDateEn(t time.Time) string {
	return t.Format("2006-01-02")
}

//Retourne une date en string au format J-M-A. Pour rapports SysACE.
func ogcDateFr(t time.Time) string {
	return t.Format("02-01-2006")
}

//fonction 'helper' pour sortir une date relative a aujourd'hui. Ceci est utilise pour les 
//rapports SysACE lorsqu'on veut, par exemple, avoir les ventes des 7 dernieres semaines.
//
//offset: nombre de jours pour lesquels on veut décaler. 
//		  Nombre positif: date dans le futur.
//		  Nombre negativ: date dans le passé.
func relativeDays(offset int) time.Time {
	return time.Now().AddDate(0, 0, offset)
}

//ogc_unload traite avec les scripts Expect l'exportation des tables Informix-SQL
//pour les charger dans la table MySQL localhost. On peut 'unloader' autant de 
//tables qu'on veut, en autant que la structure de la table existe deja sur MySQL.
//A chaque Unload, la de MySQL sera effacé pour être remplacé par l'exportation
//Informix-sql.
//
//tables: {"rev1", "rev2", ....}
func ogc_unload(tables []string) {
	pwd, _ := os.Getwd()

	err := os.Chdir("./ogc")
	if err != nil {
		log("Erreur de changement de répertoire: ", err.Error())
	}
	for _, table := range tables {
		log(fmt.Sprintf("OGC: Extraction de la table %s...", table))
		//(shell_exec(EXPECTBIN." ".OGCDUMPDIR."/ogcunload.ex ".$table)
		cmd := exec.Command("./ogcunload.ex", table)
		err := cmd.Start()
		if err != nil {
			log("Erreur de lancement de commande exportation: ", err.Error())
		}
		err = cmd.Wait()
		if err != nil {
			log("Erreur? ->", err.Error())
		}
		exists, _ := FileExists(fmt.Sprintf("inet_%s.out", table))
		if exists == false {
			log("Erreur, la base de données pour ", table, " n'a pas ete transféré sur le serveur local.")
			continue
		} else {
			log(fmt.Sprintf("MySQL: importation de %s...", table))
			cmd := exec.Command("./mysqladd.ex", table)
			err := cmd.Start()
			if err != nil {
				log("Erreur de lancement de commande exportation: ", err.Error())
			}
			err = cmd.Wait()
			if err != nil {
				log("Erreur? ->", err.Error())
			}
		}
	}
	os.Chdir(pwd)
}

//sendEMail() envoit un ou plusieurs e-mail pour un meme rapport.
//
//titre: Titre du rapport ET du sujet du e-mail
//rapport: contenu du rapport en soi
//emails: array de string contenant chacun un addresse e-mail.
//extension: si non-vide, sera une piece jointe au e-mail (txt, html, csv). Si vide, 
//affiché tel-quel dans le e-mail.
func sendEMail(titre string, rapport string, emails []string, extension string) error {
	m := NewMessage(titre, "")
	m.To = emails

	//Il m'est impossible de specifier, comme header "From", une valeur longue tel 
	//"Rapports Automatisés Rona L'Entrepot Sherbrooke <informatique@ronasherbrooke.com>",
	//car Go englobe ma variable entre <>'s . Voir
	//            http://golang.org/src/pkg/net/smtp/smtp.go#L225
	// et ensuite http://golang.org/src/pkg/net/smtp/smtp.go#L180
	m.From = "informatique@ronasherbrooke.com"

	/*
	   switch strings.ToLower(extension) {
	       case "html", "htm", "txt": //HTML et plain text a meme l'e-mail
	           m.InlineHTML = true
	           m.Attachments[fmt.Sprintf("%s.%s", titre, extension)] = []byte(fmt.Sprintf("<html><head></head><body><pre style=\"font-family: 'Courier New';font-size: 9pt;\">\n%s\n</pre></body></html>", rapport))
	       case "text": //Cas special ou on veut avoir une pièce jointe
	           m.InlineHTML = true
	           extension = "txt"
	           m.Attachments[fmt.Sprintf("%s.%s", titre, extension)] = []byte(fmt.Sprintf("%s", rapport))
	       case "": //La balance est considéré pièce jointe
	           m.InlineHTML = false
	           extension = "txt"
	           m.Attachments[fmt.Sprintf("%s.%s", titre, extension)] = []byte(fmt.Sprintf("%s", rapport))
	       default:
	   }
	*/

	//if strings.ToLower(extension) == "html" || strings.ToLower(extension) == "htm" || strings.ToLower(extension) == "txt" {
	if extension == "" { //Affiché tel quel.
		m.InlineHTML = true
		m.Attachments[fmt.Sprintf("%s.%s", titre, extension)] = []byte(fmt.Sprintf("<html><head></head><body><pre style=\"font-family: 'Courier New';font-size: 9pt;\">\n%s\n</pre></body></html>", rapport))
	} else { //Puisqu'un extension a été mentionné, attacher en pièce jointe.
		m.InlineHTML = false
		m.Attachments[fmt.Sprintf("%s.%s", titre, extension)] = []byte(fmt.Sprintf("%s", rapport))
	}

	return SendUnencrypted("10.100.2.1:25", "service.sherbrooke@rona.ca", "Public05787mail", m)
}

//ogc_sql() rapatrie un rapport SQL du système OGC, et la traite pour
//un renvoi en  e-mail ou sur une imprimante sur OGC.  Dans le cas d'un
//rapport envoyé sur imprimante, l'impression sera fait à partir d'OGC
//directement. 
//
//imprimante = numéro d'imprimante OGC. Si 'imprimante' = 0 , le
//rapport sera plutôt extrait d'OGC pour  être envoyé en e-mail.
//
//rapports = array contenant des objects Rapport (defini dans cron.go)
//attachmentExtension = si autre chose que "", le rapport sera envoyé 
//en tant que pièce jointe. Autrement, le rapport sera affiché sur écran.
func ogc_sql(imprimante string, rapports []Rapport, attachmentExtension string) { //ogc_sql_pipe()
	//BUG: les SQL ayant des dates hard-coded echouent. Impossible de contourner.
	separation := "ZX_XZseparationZX_XZ"

	rapport_expect_string := ""
	if imprimante == "0" {
		log("Rapatriement des SQL suivants: ")
	} else {
		log(fmt.Sprintf("Impression des SQL suivants (ils seront imprimés sur l'imprimante OGC %s): ", imprimante))
	}
	for i, rapport := range rapports {
		log(fmt.Sprintf("\t%s", rapport.Titre))
		rapport_expect_string += rapport.Parametres
		if i < (len(rapports) - 1) {
			rapport_expect_string += ";"
		}
	}

	//log(fmt.Sprintf("Chaine: %s",rapport_expect_string))
	cmd := exec.Command("ogc/ogcunload.ex", "ogc_sql_batch_pipe", imprimante, separation, rapport_expect_string)
	out, _ := cmd.Output()

	if imprimante != "0" { //Les rapports ont été sortis sur imprimante.
		return
	}
	s := strings.Split(string(out), separation)

	//On enleve la derniere separation a la fin de la reponse d'OGC rajoutant
	//un element vide dans notre array.
	s = s[0 : len(s)-1]

	for i, rapport := range s {
		err := sendEMail(rapports[i].Titre, rapport, rapports[i].EMail, attachmentExtension)
		if err != nil {
			fmt.Sprintf("Echec de l'envoi de l'e-mail: ", err.Error())
		}
	}
}

//ogc_rapport() rapatrie un rapport SysACE du système OGC, et la traite pour
//un renvoi en  e-mail ou sur une imprimante sur OGC.  Dans le cas d'un
//rapport envoyé sur imprimante, l'impression sera fait à partir d'OGC
//directement. 
//
//imprimante = numéro d'imprimante OGC. Si 'imprimante' = 0 , le
//rapport sera plutôt extrait d'OGC pour  être envoyé en e-mail.
//
//rapports = array contenant des objects Rapport (defini dans cron.go)
//attachmentExtension = si autre chose que "", le rapport sera envoyé 
//en tant que pièce jointe. Autrement, le rapport sera affiché sur écran.
func ogc_rapport(imprimante string, rapports []Rapport, attachmentExtension string) { //ogc_sysace_pipe()
	separation := "ZX_XZseparationZX_XZ"

	rapport_expect_string := ""
	if imprimante == "0" {
		log("Rapatriement des rapports suivants: ")
	} else {
		log(fmt.Sprintf("Impression des rapports suivants (ils seront imprimés sur l'imprimante OGC %s): ", imprimante))
	}
	for i, rapport := range rapports {
		log(fmt.Sprintf("\t%s", rapport.Titre))
		rapport_expect_string += rapport.Parametres
		if i < (len(rapports) - 1) {
			rapport_expect_string += ";"
		}
	}

	//log(fmt.Sprintf("Chaine: %s",rapport_expect_string))
	cmd := exec.Command("ogc/ogcunload.ex", "ogc_ace_batch_pipe", imprimante,
		fmt.Sprintf("%s", separation), fmt.Sprintf("%s", rapport_expect_string))
	out, _ := cmd.Output()

	if imprimante != "0" { //Les rapports ont été sortis sur imprimante.
		return
	}
	s := strings.Split(string(out), separation)

	//On enleve la derniere separation a la fin de la reponse d'OGC rajoutant
	//un element vide dans notre array.
	s = s[0 : len(s)-1]

	for i, rapport := range s {
		err := sendEMail(rapports[i].Titre, rapport, rapports[i].EMail, attachmentExtension)
		if err != nil {
			fmt.Sprintf("Echec de l'envoi de l'e-mail: ", err.Error())
		}

		/*
		   m := rona.NewMessage(rapports[i].Titre, "")
		   m.To = rapports[i].EMail

		   //Il m'est impossible de specifier, comme header "From", une valeur longue tel 
		   //"Rapports Automatisés Rona L'Entrepot Sherbrooke <informatique@ronasherbrooke.com>",
		   //car Go englobe ma variable entre <>'s . Voir
		   //            http://golang.org/src/pkg/net/smtp/smtp.go#L225
		   // et ensuite http://golang.org/src/pkg/net/smtp/smtp.go#L180
		   m.From = "informatique@ronasherbrooke.com"

		   if strings.ToLower(attachmentExtension) == "html" || strings.ToLower(attachmentExtension) == "htm" {
		       m.InlineHTML = true
		       m.Attachments[fmt.Sprintf("%s.%s", ogcDateFr(time.Now()), attachmentExtension)] = []byte(fmt.Sprintf("<html><head></head><body><pre style=\"font-family: 'Courier New';font-size: 9pt;\">\n%s\n</pre></body></html>", rapport))
		   } else {
		       m.InlineHTML = false
		       m.Attachments[fmt.Sprintf("%s-%s.%s", rapports[i].Titre, ogcDateFr(time.Now()), attachmentExtension)] = []byte(fmt.Sprintf("%s", rapport))
		   }
		   //m.InlineHTML = (strings.ToLower(attachmentExtension) == "html" || strings.ToLower(attachmentExtension) == "htm")


		   //err = rona.Send("10.100.2.1", smtp.PlainAuth("", "service.sherbrooke@rona.ca", "Public05787mail", "10.100.2.1"), m)
		   err := rona.SendUnencrypted("10.100.2.1:25", "service.sherbrooke@rona.ca", "Public05787mail", m)
		   if err != nil {
		       fmt.Sprintf("Well, that failed: ", err.Error())
		   }
		*/
	}
}

//Cette fonction doit être appelé quotidiennement, car les demandes de
//rapports automatisés seront faits à partir de cette fonction. Voir 
//le code source pour comprendre la logique.
func ogc_transfert() {

	ventestemp := Rapport{Titre: "Test SQL dans l'email éèê", Parametres: "doc_03", EMail: []string{"eric.belanger@ronasherbrooke.com"}}
	ogc_sql("0", []Rapport{ventestemp}, "")

	return

	/*    REAL CODE BELOW     */

	//Exécution des rapports quotidiens:
	sansCodeUPC := Rapport{Titre: "Liste des items sans code UPC", Parametres: "inv_rona/aerr_inv 1114 \"%\" \"\" \"97\" \"O\"", EMail: []string{"info.05787@rona.ca"}}
	horsIntervalles := Rapport{Titre: "Transactions marge hors-intervalles", Parametres: "car/arevdc2 1114 \"1\" \"%\" \"%\" \"" + ogcDateEn(relativeDays(-1)) + "\" \"" + ogcDateEn(relativeDays(-1)) + "\" 1 90 \"\" \"ZZZZZZZZZZ\"", EMail: []string{"contr.05787@rona.ca", "compt.05787@rona.ca", "dga.05787@rona.ca", "info.05787@rona.ca"}}
	ventes := Rapport{Titre: "Analyse des ventes par division du " + ogcDateFr(time.Now()), Parametres: "car/aanal_maj3 1114 \"" + ogcDateEn(relativeDays(-1)) + "\" \"" + ogcDateEn(relativeDays(-1)) + "\" \"\" \"ZZZ\" \"%\"", EMail: []string{"dms.05787@rona.ca", "dop.05787@rona.ca", "dmm.05787@rona.ca", "dga.05787@rona.ca", "dg.05787@rona.ca", "paie.05787@rona.ca", "info.05787@rona.ca", "srh.05787@rona.ca", "contr.05787@rona.ca", "compt.05787@rona.ca"}}
	ogc_rapport("0", []Rapport{sansCodeUPC, horsIntervalles, ventes}, "")

	switch int(time.Now().Weekday()) {
	case 0: //Dimanche

	case 1: //Lundi
		// Rapport E-Mail:
		prixMaison := Rapport{Titre: "Liste de verification - Changements de prix maison",
			Parametres: "sys1/MAGASIN/rs_prixchk \"" + ogcDateEn(relativeDays(-7)) + "\" \"" + ogcDateEn(relativeDays(-1)) + "\" ",
			EMail:      []string{"info.05787@rona.ca", "contr.05787@rona.ca", "dga.05787@rona.ca"}}
		ventesSemaine := Rapport{Titre: "Analyse des ventes de la semaine passée",
			Parametres: "car/aanal_maj3 1114 \"" + ogcDateEn(relativeDays(-7)) + "\" \"" + ogcDateEn(relativeDays(-1)) + "\" \"\" \"ZZ\" \"%\"",
			EMail:      []string{"info.05787@rona.ca", "dmm.05787@rona.ca", "dga.05787@rona.ca", "dmm.05787@rona.ca", "dms.05787@rona.ca", "dop.05787@rona.ca"}}
		ventesCaisse := Rapport{Titre: "Analyse des ventes par heure par caisse",
			Parametres: "car/atemp 1114 \"%\" \"%\" \"" + ogcDateEn(relativeDays(-7)) + "\" \"" + ogcDateEn(relativeDays(-1)) + "\" \"%\"",
			EMail:      []string{"info.05787@rona.ca", "dga.05787@rona.ca"}}
		fraisLivraison := Rapport{Titre: "Liste des livraisons par frais",
			Parametres: "sys1/MAGASIN/inet_liv1",
			EMail:      []string{"info.05787@rona.ca", "contr.05787@rona.ca", "compt.05787@rona.ca"}}
		ogc_rapport("0", []Rapport{prixMaison, ventesSemaine, ventesCaisse, fraisLivraison}, "")

		// Sortie sur la 1000:
		journalPrix := Rapport{Titre: "Journal de verification de prix (semaine)",
			Parametres: "inv_rona/ajver_prix 1114 \"" + ogcDateEn(relativeDays(-7)) + "\" \"" + ogcDateEn(relativeDays(-1)) + "\" \"\" \"ZZZZZZZZZZ\" \"O\" 1",
			EMail:      []string{""}}
		limiteCredit := Rapport{Titre: "Liste des modification des limites de credit",
			Parametres: "car/alim_aug 1114 \"\" \"ZZZZZZZZ\" \"" + ogcDateEn(relativeDays(-7)) + "\" \"" + ogcDateEn(relativeDays(-1)) + "\"",
			EMail:      []string{""}}
		ogc_rapport("1000", []Rapport{ventesSemaine, journalPrix, limiteCredit}, "")

		// Sortie sur la 1021:
		stocksEnt2 := Rapport{Titre: "Stocks en main par produit (Entrepot 2)",
			Parametres: "inv_rona/aval_cour 1114 \"\" \"ZZZZZZZZZZZZZZZ\" 2 2 \"M\" \"O\" \"\" \"Z\" ",
			EMail:      []string{""}}
		ogc_rapport("1021", []Rapport{stocksEnt2}, "")

		// E-Mail (Pieces jointes)
		exceptionsCSV := Rapport{Titre: "Liste d'exceptions de prix",
			Parametres: "sys1/MAGASIN/aexcep_csv \"" + ogcDateEn(relativeDays(-7)) + "\" \"" + ogcDateEn(relativeDays(-1)) + "\" \"%\" \"%\" \"1\" \"1\"",
			EMail:      []string{"info.05787@rona.ca"}}
		ogc_rapport("0", []Rapport{exceptionsCSV}, "csv")

		// Rapports de commande d'achats pour Jacynthe
		caNCMoisCourant := Rapport{Titre: "C.A. non-completés pour le mois courant", Parametres: "PB_CANCmc", EMail: []string{"info.05787@rona.ca", "e3.05787@rona.ca"}}
		caNCAnneePrecedente := Rapport{Titre: "C.A. non-completés pour l'année passée", Parametres: "PB_CANC2k7", EMail: []string{"info.05787@rona.ca", "e3.05787@rona.ca"}}
		caNCAnneeCourante := Rapport{Titre: "C.A. non-completés pour l'année courante", Parametres: "PB_CANC2k8", EMail: []string{"info.05787@rona.ca", "e3.05787@rona.ca"}}
		ogc_sql("9215", []Rapport{caNCMoisCourant, caNCAnneePrecedente, caNCAnneeCourante}, "")

		retoursSF := Rapport{Titre: "Liste des CV \"Retours sans factures\"", Parametres: "inet_sf", EMail: []string{"info.05787@rona.ca", "contr.05787@rona.ca"}}
		ogc_sql("0", []Rapport{retoursSF}, "")

	case 2: //Mardi
		// PB_21
		pb21 := Rapport{Titre: "Liste des produits en code de changement de prix #21", Parametres: "PB_21", EMail: []string{"info.05787@rona.ca", "dga.05787@rona.ca", "dop.05787@rona.ca", "dmm.05787@rona.ca", "dms.05787@rona.ca"}}
		ogc_sql("0", []Rapport{pb21}, "")

		//Inventaire des caisses
		invcaisse := Rapport{Titre: "Inventaire des caisses", Parametres: "invcaisse", EMail: []string{"info.05787@rona.ca"}}
		ogc_sql("1000", []Rapport{invcaisse}, "")
		ogc_sql("9215", []Rapport{invcaisse}, "")

		// Stocks pour l'entrepot 2 pour Manon
		stocksEnt2 := Rapport{Titre: "Stocks en main par produit (Entrepot 2)",
			Parametres: "inv_rona/aval_cour 1114 \"\" \"ZZZZZZZZZZZZZZZ\" 2 2 \"M\" \"O\" \"\" \"Z\" ",
			EMail:      []string{""}}
		ogc_rapport("1001", []Rapport{stocksEnt2}, "")

		// Liste des SAVs pour Sylvie
		savQteEnMain := Rapport{Titre: "Liste des codes SAV avec quantité en main (2 dernières semaines)", Parametres: "inet_sav", EMail: []string{"info.05787@rona.ca", "contr.05787@rona.ca", "inv.05787@rona.ca"}}
		ogc_sql("0", []Rapport{savQteEnMain}, "")

	case 3: //Mercredi
		circulaire := Rapport{Titre: "Liste des produits descendus par Boucherville dans la prochaine circulaire (SD)",
			Parametres: "inv_rona/apritel 1114 \"" + ogcDateEn(relativeDays(+8)) + "\" \"%\" 1 \"SD\" \"SD\" \"2\" \"\" \"ZZZZZZZZ\" \"\" \"99\" \"O\" \"N\"",
			EMail:      []string{"info.05787@rona.ca", "dga.05787@rona.ca", "dms.05787@rona.ca", "dmm.05787@rona.ca", "dop.05787@rona.ca"}}
		ogc_rapport("1005", []Rapport{circulaire}, "")

	case 4: //Jeudi
		//fmt.Println("Jeudi")
	case 5: //Vendredi
		//fmt.Println("Vendredi")
	case 6: //Samedi
		//fmt.Println("Samedi")
	}
}

//Récupère les ventes d'OGC. OGC de son côté appelle IBM pour obtenir les ventes.
//Cette fonction retourne le rapport de ventes 'live' convenablement formatté
func ventes_IBM_fetch() string {
	//Lance le script expect qui interroge OGC via rf80.
	cmd := exec.Command("ogc/ogcunload.ex", "gsa_ventes_online")
	outBytes, _ := cmd.Output()
	outExpect := string(outBytes) //byte en string[]

	/*
	   //Testing avec un fichier local: 
	   outBytes, err := ioutil.ReadFile("ventes.txt")
	   if err != nil {
	       fmt.Println("Erreur de lecture: ", err.Error())
	       os.Exit(1)
	   }
	   outExpect := string(outBytes) //byte en string[]
	*/

	//L'exportation du systeme IBM laisse telement d'escape characters que ça
	//en est vraiment pas drôle. Le format d'IBM est très weird (allez voir le
	//fichier exporté dans Hex Editor pour le fun!).  On epure le grabuge ici
	//(ne PAS modifier l'ordre du array sans faire de tests approfondies, car
	//l'ordre est très important!):
	garbage := []string{"\x1b@1\x05", "\x1b", "\x05", "\x0f", "\x18", "\x03", "\x0e", "\x17", "@12", "@1", "\r"}
	for _, g := range garbage {
		outExpect = strings.Replace(outExpect, g, "", -1)
	}
	cleanedSlice := strings.Split(outExpect, "\n")
	cleanedSlice = cleanedSlice[5 : len(cleanedSlice)-2]

	if len(cleanedSlice) > 3 {
		//3 lignes dans l'array est le minimum de lignes  possible (magasin
		//tout juste ouvert et AUCUNE vente à afficher)

		rapport := ""
		//regexp.Compile va englober les valeurs recherchés entre parenthèses
		//sur la ligne result_slice plus bas.
		r, err := regexp.Compile("([0-9]{2})\\s*([0-9]*)\\s*([0-9.]*)\\s%#\\s*([0-9.]*)\\s*([0-9.]*)\\s%")
		if err == nil {
			rapport += fmt.Sprintf("\tNombre\tPourcent\t\t\tPourcent\n")
			rapport += fmt.Sprintf("Dept\tItems\tItems\t  Ventes\t\tVentes\n")
			for _, l := range cleanedSlice[:len(cleanedSlice)-2] {
				result_slice := r.FindAllStringSubmatch(l, -1)
				line := ""
				for i := 1; i < len(result_slice[0])+1; i++ {
					switch i {
					case 1: //Departement
						line += fmt.Sprintf("%2s\t", result_slice[0][i])
					case 2: //Transactions # (items scannés)
						line += fmt.Sprintf("%4s\t", result_slice[0][i])
					case 3: //Transactions %
						line += fmt.Sprintf("%5s%%\t", result_slice[0][i])
					case 4: //Ventes $
						line += fmt.Sprintf("%8s$\t", result_slice[0][i])
					case 5: //Ventes %
						line += fmt.Sprintf("%5s%%", result_slice[0][i])
					}
				}
				rapport += fmt.Sprintf("%s\n", line)
			}
		} //Else ignoré: compilation de mon regex marche.

		rTotal, errTotal := regexp.Compile("Total\\s([0-9]*)\\s#\\s\\$([0-9.]*)")
		if errTotal == nil {
			total_slice := rTotal.FindAllStringSubmatch(cleanedSlice[len(cleanedSlice)-2], -1)
			rapport += fmt.Sprintf("\nTotal d'items: %s\tTotal ventes: %s$\n", total_slice[0][1], total_slice[0][2])
		}
		return rapport
	}
	return "Pas de données de ventes (système de caisses fermé, ou trop tôt dans la matinée)"
}

//Envoie en e-mail les statistiques de vente de la journée. 
//emails est un array d'addresses e-mail.
func ventes_IBM(emails []string) {
	ventes := ventes_IBM_fetch()
	now := fmt.Sprintf(time.Now().Format("02-01-2006, 15:04:05"))
	err := sendEMail("Chiffres de ventes (à "+now+")", ventes, emails, "")
	if err != nil {
		fmt.Println("Erreur d'envoi d'e-mail :", err.Error())
	}
}

//Cron() est lancé à partir de la ligne de commande en shell pour exécuter nos
//commandes, généralement via crontab de nuit
//Présentement, deux usages sont possibles:
//
// `programme action=ogc_transfert`: exécute les exportations de la DB, ainsi que
//                                   l'extraction de rapports quotidiens depuis OGC
//
// `programme action=ogc_ventes` :  Rapatrie les ventes d'IBM pour envoi aux 
//									directeurs.
// `programme action=ogc_gratteux` :  	Alias de ogc_ventes qui envoit un e-mail a 
//										Renelle, Jean, et Informatique.

func Cron() {
	if len(os.Args) == 1 {
		usage := `Usage: <executable> <ogc_transfert>|<ogc_rapport_ventes_online>

	ogc_transfert == Transfert quotidien des tables Informix d'OGC sur MySQL (Hosted local)
	ogc_ventes == Fetch des ventes d'IBM pour envoi aux directeurs.
		`
		fmt.Println(usage)
	}
	for _, v := range os.Args {
		if strings.Index(v, "action=") == 0 { //si 'action=' commence le string...
			action := strings.Split(v, "=")
			switch {
			case len(action) == 1:
				os.Exit(0)
			case action[1] == "ogc_transfert":
				ogc_transfert()

			case action[1] == "ogc_ventes":
				//SVP Entrer les e-mails un a la suite de l'autre. 
				ventes_IBM([]string{
					"info.05787@rona.ca",
					"renelle.anctil@ronasherbrooke.com",
				})
			case action[1] == "ogc_gratteux":
				//SVP Entrer les e-mails un a la suite de l'autre. 
				ventes_IBM([]string{
					"informatique@ronasherbrooke.com",
					"dga.05787@rona.ca",
					"renelle.anctil@ronasherbrooke.com",
				})
			}
		}
	}

}
