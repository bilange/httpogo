/*
 *
 * Emulation backend.php
 *
 */

package rona

import (
	"encoding/xml"
	"fmt"
	"github.com/ziutek/mymysql/mysql"
	_ "github.com/ziutek/mymysql/native"
	"os"
	"strings"
)

//Sauvegarde un nouveau prix enregistre dans le fichier Excel.
//Ceci est pour mettre un peu de 'gras' autour de l'os sur le raisonnement
//sur le changement de prix. Ceci est une fonctionnalite utilise dans le fichier
//excel de changement de prix.
func Backend_prix_add() {
	//action=prix_add&coderona=xxx&um=xx&date=x&prix=x&codechg=x&notes=xxx

	//ATTENTION cette function utilise un hack pour voir autant "&force" que
	//"force=1" dans l'url

	HttpSetContentTypeUTF("text/plain")

	codeRona, exists := GetHTTPArgument("coderona")
	if exists == false {
		HttpSetResponse("Pas de code Rona specifié.")
		FlushHttp(1)
	}

	descr, exists := GetHTTPArgument("um")
	if exists == false {
		HttpSetResponse("Pas d'unité de mesure specifié.")
		FlushHttp(1)
	}

	date, exists := GetHTTPArgument("date")
	if exists == false {
		HttpSetResponse("Pas de date specifié.")
		FlushHttp(1)
	}

	um, exists := GetHTTPArgument("um")
	if exists == false {
		HttpSetResponse("Pas d'unité de mesure de specifié.")
		FlushHttp(1)
	}

	prix, exists := GetHTTPArgument("prix")
	if exists == false {
		HttpSetResponse("Pas de prix specifié.")
		FlushHttp(1)
	}

	codeChangement, exists := GetHTTPArgument("codechg")
	if exists == false {
		HttpSetResponse("Pas de code de changement de prix specifié.")
		FlushHttp(1)
	}

	notes, exists := GetHTTPArgument("notes")
	if exists == false {
		HttpSetResponse("Pas de notes de specifié.")
		FlushHttp(1)
	}

	codeRona = SQLSafeEscape(codeRona)
	descr = SQLSafeEscape(descr)
	date = SQLSafeEscape(date)
	prix = SQLSafeEscape(prix)
	codeChangement = SQLSafeEscape(codeChangement)
	notes = SQLSafeEscape(notes)

	db := mysql.New("tcp", "", "127.0.0.1:3306", "gesadm", "mdaseg", "intranet")

	err := db.Connect()
	if err != nil {
		XMLDie(fmt.Sprint("Echec de connexion: ", err))
	}

	existRow, _, existError := db.Query("select * from  prixnotes where rona='%s' and datedebut='%s' and um='%s' and prix='%s'", codeRona, date, um, prix)
	if existError != nil {
		XMLDie(fmt.Sprint("Echec de connexion: ", existError))
	}
	_, forceArg := GetHTTPArgument("force")
	forceNArg := false

	//Le fichier Excel a un bug (toujours present en version 6 du fichier) dans
	//lequel l'URL pour "&force=1" n'incluais pas le '&', ce qui n'etait pas
	//toujours catché par PHP. Ceci est un workaround
	fullargs, _ := GetKVP(GetEnvironmentVariables(), "QUERY_STRING")
	if strings.Contains(fullargs, "force=1") {
		forceNArg = true //NArg = 'not' argument
	}

	if (len(existRow) > 0 && (forceArg == true || forceNArg == true)) || len(existRow) == 0 { //update
		sql := fmt.Sprintf("insert into prixnotes values('','%s','%s','%s','%s','','%s','%s');", codeRona, prix, notes, date, um, codeChangement)
		_, _, err = db.Query(sql)

		HttpContentType = "text/plain"
		HttpWriteHeader("Expires: Thu, 01 Dec 1994 16:00:00 GMT\n")
		HttpWriteHeader("Server: Golang standalone CGI script\n")
		if err != nil {
			HttpSetResponse(fmt.Sprintf("erreur: %s", err))
		} else {
			HttpSetResponse("success")
		}
		FlushHttp(0)
	} else if len(existRow) > 0 && forceArg == false && forceNArg == false {
		HttpContentType = "text/plain"
		HttpWriteHeader("Expires: Thu, 01 Dec 1994 16:00:00 GMT\n")
		HttpWriteHeader("Server: Golang standalone CGI script\n")
		HttpSetResponse("double")
		FlushHttp(0)
	}
	HttpContentType = "text/plain"
	HttpWriteHeader("Expires: Thu, 01 Dec 1994 16:00:00 GMT\n")
	HttpWriteHeader("Server: Golang standalone CGI script\n")
	HttpSetResponse("huh?") //ne devrait pas arriver ici.
	FlushHttp(0)

}

//Affiche un historique (en HTML) de prix pour un produit donne.
//Est utilise dans le logiciel de changement de prix.
func Backend_prix_historique() {
	//action=prix_historique&code=99955000

	codeRona, exists := GetHTTPArgument("code")
	if exists == false {
		XMLDie("Pas de code Rona specifié.")
	}
	codeRona = SQLSafeEscape(codeRona)

	db := mysql.New("tcp", "", "127.0.0.1:3306", "gesadm", "mdaseg", "ogc")
	err := db.Connect()
	if err != nil {
		XMLDie(fmt.Sprint("Echec de connexion: ", err))
	}

	HttpSetContentTypeUTF("text/html")
	listRow, listRes, listErr := db.Query("select pr1_in1_code,pr1_date_deb,pr1_date_fin,pr1_unit_vente,pr1_prix_vente1,pr1_code,ra2_desc from prix1,raison2 where pr1_in1_code='%s' and pr1_code=ra2_code_raison order by pr1_date_deb desc;", codeRona)
	if listErr != nil {
		fmt.Println("\n\nEchec dans la requête SQL sur la liste de produits: ", listErr)
		os.Exit(1)
	}
	if len(listRow) == 0 {
		HttpSetResponse("<html><body>Pas d'historique de prix pour ce produit. (??)</body></html>")
		FlushHttp(0)
	}
	HttpSetResponse(`<html>
<head>
<link rel="stylesheet" href="/css/screen.css" type="text/css" media="screen" /> 
<link rel="stylesheet" href="/css/decoration.css" type="text/css" media="screen" /> 
</head>
<body>
<script type="text/javascript" src="/js/jquery-1.2.js"></script> 
<script type="text/javascript" src="/js/jquery.firebug.debug.js"></script>
<script type="text/javascript" src="/js/jquery.tablesorter.js"></script>
<table width="100%" id="rapport" cellspacing="0" cellpadding="5" border="0" style="border: 0px solid #9f9f9f;">
<thead><tr><th>Date</th><th>Fin</th><th>U/V</th><th>Prix</th><th>Code</th><th>Raison (OGC)</th><th>Explication</th></tr></thead><tbody>
`)
	dateDebut := ""
	dateFin := ""
	uv := ""
	prix := ""
	code := ""
	raisonOGC := ""
	raisonCustom := ""
	raisonCustomSQLPart := ""
	dbIntranet := mysql.New("tcp", "", "127.0.0.1:3306", "gesadm", "mdaseg", "intranet")
	err = dbIntranet.Connect()
	if err != nil {
		XMLDie(fmt.Sprint("Echec de connexion: ", err))
	}
	for _, row := range listRow {
		dateDebut = row.Str(listRes.Map("pr1_date_deb"))
		dateFin = row.Str(listRes.Map("pr1_date_fin"))
		uv = row.Str(listRes.Map("pr1_unit_vente"))
		prix = row.Str(listRes.Map("pr1_prix_vente1"))
		code = row.Str(listRes.Map("pr1_code"))
		raisonOGC = row.Str(listRes.Map("ra2_desc"))

		switch code {
		case "20", "23", "24", "25":
			raisonCustomSQLPart = " and code in ('20','23','24','25') "
		case "SD", "":
			raisonCustomSQLPart = " and code in ('SD','') "
		default:
			raisonCustomSQLPart = ""
		}

		raisonMRow, raisonMRes, raisonMErr := dbIntranet.Query("select * from prixnotes where rona='%s' and datedebut='%s' and um='%s' %s", codeRona, dateDebut, uv, raisonCustomSQLPart)
		if raisonMErr != nil || len(raisonMRow) == 0 {
			raisonCustom = "&nbsp;"
		} else {
			raisonCustom = raisonMRow[0].Str(raisonMRes.Map("raison"))
		}
		HttpWriteResponse(fmt.Sprintf("\t<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n", dateDebut, dateFin, uv, prix, code, raisonOGC, raisonCustom))
	}
	HttpWriteResponse(`</tbody></table> <script type="text/javascript"> $(document).ready(function() { $('table').tablesorter(); });</script></body></html>`)
	HttpWriteResponse("\n")
	FlushHttp(0)
}

//Sauvegarde les informations de description de produit qui serait
//a mettre a jour. L'usager du logiciel d'affiches a change la description, et
//il nous a propose d'updater la description.
//Semblerait que le script PHP ne redonnait aucunement d'informations (en XML) 
//pour le client. Donc je n'en ai pas rajouté.
func Backend_productinfo_updatedesc() { //action=productinfo_updatedesc&rona=99955000&descr=Chaloupe

	codeRona, exists := GetHTTPArgument("rona")
	if exists == false {
		XMLDie("Pas de code Rona specifié.")
	}
	descr, exists := GetHTTPArgument("descr")
	if exists == false {
		XMLDie("Pas de description specifié.")
	}
	codeRona = SQLSafeEscape(codeRona)
	descr = SQLSafeEscape(descr)

	db := mysql.New("tcp", "", "127.0.0.1:3306", "gesadm", "mdaseg", "ogc")

	err := db.Connect()
	if err != nil {
		XMLDie(fmt.Sprint("Echec de connexion: ", err))
	}

	existRow, _, existError := db.Query("select * from sidma_technique where rona='%s';", codeRona)
	if existError != nil {
		XMLDie(fmt.Sprint("Echec de connexion: ", existError))
	}
	if len(existRow) > 0 { //update
		db.Query("update sidma_technique set sidma_technique.desc='%s',custom=1 where rona='%s';", descr, codeRona)
	} else { //insert
		sql := fmt.Sprintf("insert into sidma_technique values('%s','%s',1);", codeRona, descr)
		db.Query(sql)
	}
	HttpContentType = "text/plain"
	HttpWriteHeader("Expires: Thu, 01 Dec 1994 16:00:00 GMT\n")
	HttpWriteHeader("Server: Golang standalone CGI script\n")
	FlushHttp(0)
}

//Retourne un XML pour etre utilisee par le logiciel d'affiches,
//ou le fichier Excel de changement de prix.
//Dans le cas d'un code Rona inexistant, la fonction retourne le même XML
//(toutes les balises seront présentes) mais vide d'information. Le logiciel
//d'affiche et Excel vérifie si le code existe en vérifiant le nombre de 
//balises <upc> sous la balise <upcs>
func Backend_productinfo() {
	//action=productinfo&code=99955000&regulierseulement
	//regulierseulement = detecte seulement "if set" ou pas
	HttpContentType = "text/xml"
	HttpWriteHeader("Expires: Thu, 01 Dec 1994 16:00:00 GMT\n")
	HttpWriteHeader("Server: Golang standalone CGI script\n")

	codeRona, exists := GetHTTPArgument("item")
	_, prixRegulierSeulement := GetHTTPArgument("regulierseulement")
	if exists == false {
		XMLDie("Pas de code Rona specifie.")
	}

	codeRona = SQLSafeEscape(codeRona)

	db := mysql.New("tcp", "", "127.0.0.1:3306", "gesadm", "mdaseg", "ogc")

	err := db.Connect()
	if err != nil {
		XMLDie(fmt.Sprint("Echec de connexion: ", err))
	}

	mainRows, mainRes, mainError := db.Query("select in1_code, in1_cout_dern_ach, in1_desc_f, in1_code_fou, in1_desc_courte, in1_desc_courte_2,in1_style_mod, in1_composante, in1_finition, in1_marq_comm from inv1,bar1 where bar_in1_code=in1_code and (in1_code='%s' OR bar_code='%s') limit 1 ", SQLSafeEscape(codeRona), SQLSafeEscape(codeRona))
	if mainError != nil {
		XMLDie(fmt.Sprint("Erreur dans le SQL: ", mainError))
	}

	produit := &ProductInfo{Rona: codeRona}
	if len(mainRows) == 0 {
		out, _ := xml.MarshalIndent(produit, "", "\t")
		HttpWriteResponse(string(out))
		FlushHttp(0)
	}

	codeRona = mainRows[0].Str(mainRes.Map("in1_code"))
	produit.Titre = ToUpperWords(OGCStringConvert(mainRows[0].Bin(mainRes.Map("in1_desc_f"))))
	produit.Desc1 = ToUpperWords(OGCStringConvert(mainRows[0].Bin(mainRes.Map("in1_desc_courte"))))
	produit.Desc2 = ToUpperWords(OGCStringConvert(mainRows[0].Bin(mainRes.Map("in1_desc_courte_2"))))
	produit.CodeProduit = mainRows[0].Str(mainRes.Map("in1_code_fou"))
	produit.Style = ToUpperWords(OGCStringConvert(mainRows[0].Bin(mainRes.Map("in1_style_mod"))))
	produit.Composante = mainRows[0].Str(mainRes.Map("in1_composante"))
	produit.Marque = mainRows[0].Str(mainRes.Map("in1_marq_comm"))
	produit.Cout = mainRows[0].Float(mainRes.Map("in1_cout_dern_ach"))

	locRows, locRes, locError := db.Query("select in2_loc_mag loc,'P' type, in2_etiq_for etiq from inv2 where in2_in1_code='%s' and in2_en1_code='1' union select lc1_loc localisation,'S' type, lc1_etiq_for etiq from loc1 where lc1_in1_code='%s' and lc1_en1_code='1';", codeRona, codeRona)
	if locError != nil {
		XMLDie(fmt.Sprint("Erreur dans le SQL localisation: ", locError))
	}
	if len(locRows) > 0 {
		for _, locRow := range locRows {
			locEtiqNo, convErr := locRow.IntErr(locRes.Map("etiq"))
			if convErr != nil {
				locEtiqNo = 0 //Erreur de conversion, probablement un etiquette au format NULL dans la db.
			}
			//produit.AddLocalisation(locRow.Str(locRes.Map("loc")), locRow.Str(locRes.Map("type")), locRow.Int(locRes.Map("etiq")))
			produit.AddLocalisation(locRow.Str(locRes.Map("loc")), locRow.Str(locRes.Map("type")), locEtiqNo)
		}
	}
	techRows, _, techError := db.Query("select sidma_technique.desc from sidma_technique where rona='%s';", codeRona)
	if techError != nil {
		XMLDie(fmt.Sprint("Erreur dans le SQL sidma_technique: ", techError))
	}
	if len(techRows) > 0 {
		produit.Technique = OGCStringConvert(techRows[0].Bin(0))
	}

	ums := make([]string, 0, 0)
	umsRows, umsRes, umsError := db.Query("select distinct bar_unit_vente from bar1 where bar_in1_code='%s';", codeRona)
	if umsError != nil {
		XMLDie(fmt.Sprint("Erreur dans le SQL UPC: ", umsError))
	}
	if len(umsRows) > 0 {
		for _, umsRow := range umsRows {
			ums = append(ums, umsRow.Str(umsRes.Map("bar_unit_vente")))
		}
	}

	upcSQL := ""
	for _, um := range ums {
		if prixRegulierSeulement == true {
			upcSQL = fmt.Sprintf("select bar_code, bar_date_dern_vent, bar_unit_vente, bar_format, pr1_code, pr1_prix_vente1, pr1_fact_conv from bar1, prix1 where bar_in1_code='%s' and bar_in1_code=pr1_in1_code and bar_unit_vente='%s' and pr1_seq=(select max(pr1_seq) from prix1 where pr1_in1_code=bar_in1_code and bar_unit_vente=pr1_unit_vente and pr1_code not in (select ra2_code_raison from raison2 where ra2_prix_reg='N' or pr1_code in (\"  \",\"SD\",null))) order by bar_date_dern_vent desc limit 1;", codeRona, um)
		} else {
			upcSQL = fmt.Sprintf("select bar_code, bar_date_dern_vent, bar_unit_vente, bar_format, pr1_code, pr1_prix_vente1, pr1_fact_conv from bar1, prix1 where bar_in1_code='%s' and bar_in1_code=pr1_in1_code and bar_unit_vente='%s' and pr1_seq=(select max(pr1_seq) from prix1 where pr1_in1_code=bar_in1_code and bar_unit_vente=pr1_unit_vente and (pr1_code between \"00\" and \"99\" OR (pr1_code in (\"  \",\"SD\",null) and CURRENT_DATE between pr1_date_deb and pr1_date_fin))) order by bar_date_dern_vent desc limit 1; ", codeRona, um)
		}
		upcRows, upcRes, upcError := db.Query(upcSQL)
		if upcError != nil {
			XMLDie(fmt.Sprint("Erreur dans le SQL UPC: ", upcError))
		}
		if len(upcRows) > 0 {
			for _, upcRow := range upcRows {
				produit.AddUPC(UPC{
					Code:       upcRow.Str(upcRes.Map("bar_code")),
					UV:         upcRow.Str(upcRes.Map("bar_unit_vente")),
					PrixCode:   upcRow.Str(upcRes.Map("pr1_code")),
					Format:     upcRow.Str(upcRes.Map("bar_format")),
					Prix:       upcRow.Float(upcRes.Map("pr1_prix_vente1")),
					Conversion: upcRow.Float(upcRes.Map("pr1_fact_conv"))})
			}
		}
	}

	pritelRows, pritelRes, pritelError := db.Query("select * from pritel where pri_in1_code='%s' order by pri_date_eff desc,pri_code", codeRona)
	if pritelError != nil {
		XMLDie(fmt.Sprint("Erreur dans le SQL pritel: ", pritelError))
	}
	if len(pritelRows) > 0 {
		for _, pritelRow := range pritelRows {
			produit.AddPritel(Pritel{
				Date:       pritelRow.Str(pritelRes.Map("pri_date_eff")),
				Datefin:    pritelRow.Str(pritelRes.Map("pri_date_fin")),
				Circulaire: pritelRow.Str(pritelRes.Map("pri_plx")),
				UV:         pritelRow.Str(pritelRes.Map("pri_unit_vente")),
				Prix:       pritelRow.Float(pritelRes.Map("pri_prix")),
				PrixCode:   pritelRow.Str(pritelRes.Map("pri_code"))})
		}
	}

	//Encoding voir: https://github.com/kisielk/gorge/blob/master/util/util.go
	out, _ := xml.MarshalIndent(produit, "", "\t")
	HttpSetContentTypeUTF("text/xml")
	HttpSetResponse(fmt.Sprintf("%s%s", xml.Header, out))
	FlushHttp(0)
}
