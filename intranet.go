/*

Reprogrammation de la partie toujours utilisée du site Intranet

Nous avions un site web dans lequel un ancien directeur voulait avoir un site
'tout en un' (formations, rapports OGC, horaire, fichiers, flash meeting),
mais dû à plusieurs contraintes (dont le temps, la résistance au changement et
les resources humaines (je ne parle pas du departement ici)), le site n'a pas
été utilisée malgré le temps de dévloppement passé dessus qui est quand même
considérable.

Pour ne pas à avoir à supporter un site web pour absolument rien (le site web
a comme pré-requis: lighttpd, PHP, et une librairie d'images pour PHP), la
partie toujours utilisée a été reprogrammé en Google Go (golang.org), pour
plusieurs raisons:

- En Go, la compilation statique est la seule façon de compiler un programme.
  Quoique ça a l'air d'un défaut, simplement copier *UN* seul fichier statique
  (pas de dépendances, librairies, etc) de 2-3 megs sur un serveur Linux
  assure la continuité des fonctionalités sans crainte de briser quoi que ce
  soit si on doit updater le serveur ou le software sur cette machine.
    - (Ceci dit, on doit quand même utiliser Expect pour l'automatisation OGC,
      (et MySQL pour la base de données -- on fait une copie locale des
      (données OGC sur ce serveur-ci)
- Le code a été épuré (un gros reset, quoi), ou seulement la partie utilisée reste. 
- Ne dépend pas d'un serveur web, php et de la librairie d'images pour
  fonctionner.
    - Ceci dit, le fichier executable est rattachable en CGI pour des raisons 
      de compatibilité avec la
      configuration actuelle. Les "clients" (le programme C# et un fichier
      excel) ont besoin de communiquer avec la page web
      http://10.6.40.10/backend.php pour des requetes XML (AJAX) -- ce
      programme réponds aux requêtes de manière identique à Intranet.
- Rapidité d'exécution (se rapproche au C et doit égaler/potentiellement
  surpasser Python)
- Rapidité de compilation (seconde(s) au pire)
- Le code ici a éte documenté amplement. De ce fait, Google Go inclus
  automatiquement la documentation du code à travers leur systeme de
  documentation. Pour l'utiliser:
    - Aller au dossier contenant les fichiers *.go. Sur le serveur 'live' en
      date d'ecrire ce document, il s'agira du dossier /var/www/go/src/rona
    - Taper la commande:     godoc -http=":6060"
    - Ceci bloquera le shell, mais aller sur http://<ip>:6060/pkg/rona depuis
      un browser web, et MAGIE, toute la doc de ce code (sauf ce bloc) est formatée
      de maniere lisible! :-) 

*/

package main

import (
	//"fmt"
	"rona"
	"strings"
)

func main() {
	if rona.IsHTTP() {
		//L'environment d'execution contient $REQUEST_URI. Ceci nous indique que l'application roule en tant que CGI. 
		//Nous devons servir une page web.

		path := strings.Split(rona.GetEnv("SCRIPT_FILENAME"), "/")
		filename := path[len(path)-1]

		//Pour le moment, ce programme répond lorsqu'on l'appelle en tant que
		//CGI script. On peut créer des liens symboliques sur les noms de
		//fichiers "backend.php" ou "cron.php" vers CE programme pour une
		//exécution différente. Ce switch définit quoi faire selon comment on 
		//appelle ce programme.
		switch filename {
		case "backend.php", "backend":
			action, exists := rona.GetHTTPArgument("action")
			if exists == false { //Affichage dans un browser de l'usage de backend.php.
				rona.HttpContentType = "text/html"
				rona.HttpWriteHeader("Expires: Thu, 01 Dec 1994 16:00:00 GMT\n")
				rona.HttpWriteHeader("Server: Golang standalone CGI script\n")

				rona.HttpWriteResponse("<h1>backend.php</h1>\n")
				rona.HttpWriteResponse("<p>Il n'y a pas de fonctions en appelant directement backend.php sans param&egrave;tres. Voici comment appeller backend.php:</p>\n")
				rona.HttpWriteResponse("<ul>\n")
				rona.HttpWriteResponse("\t<li><a href=\"?action=productinfo&item=3414069\">XML: information sur un code Rona</a></li>\n")
				rona.HttpWriteResponse("\t<li><a href=\"?action=productinfo_updatedesc&rona=99995000&descr=Chaloupe\">XML: Upload d'une nouvelle description de produit</a></li>\n")
				rona.HttpWriteResponse("\t<li><a href=\"?action=prix_historique&code=99955000\">HTML: Historique d&eacute;taill&eacute;e d'un code Rona</a></li>\n")
				rona.HttpWriteResponse("\t<li><a href=\"?action=prix_add&coderona=99955000&um=CH&date=2012-12-31&prix=9.99&codechg=20&notes=Canac+Marquis\">XML: ajout d'une trace sur un nouveau prix ajout&eacute; sur un produit.</a> Ceci sera appel&eacute; et utilis&eacute; par le listing &agrave; <i>backend.php?action=prix_historique&code=...</i></li>\n")
				rona.HttpWriteResponse("</ul>\n")
				rona.HttpWriteResponse("<p>XML/HTML dans la liste ci-haut indique quelle genre de r&eacute;ponse du serveur vous devriez avoir suite &agrave; une de ces requ&ecirc;tes.</p>\n")
				rona.HttpWriteResponse("<p>Aussi voir <a href=\"cron.php\">cron.php</a>.</p>\n")
				rona.FlushHttp(0)
				return
			} else {
				switch action {
				case "productinfo":
					rona.Backend_productinfo()
				case "productinfo_updatedesc":
					rona.Backend_productinfo_updatedesc()
				case "prix_historique":
					rona.Backend_prix_historique()
				case "prix_add":
					rona.Backend_prix_add()
				case "dev_message": //desuet, mais le fichier excel et d'affiches peuvent
					return //nous lancer ce message en cas d'erreur pour bug report.
				}
			}

		case "cron.php":
			rona.HttpContentType = "text/html"
			rona.HttpWriteHeader("Expires: Thu, 01 Dec 1994 16:00:00 GMT\n")
			rona.HttpWriteHeader("Server: Golang standalone CGI script\n")
			response := `
<h1>cron.php</h1>
<p>Du point de vue de l'ancien site Intranet, cron.php &eacute;tait appel&eacute; via la ligne de commande de fa&ccedil;on suivante:</p>
<blockquote style="font-family: Courier;">/usr/local/bin/php-cgi -f /var/www/cron.php action=ogc_transfert</blockquote>
<p>Ceci dit, cron.php a &eacute;t&eacute; compil&eacute; en fichier ex&eacute;cutable. On peut maintenant appeler directement le programme (en command-line), tel que:</p>
<blockquote style="font-family: Courier;">/chemin/vers/cron.php action=ogc_transfert</blockquote><br />
(En r&eacute;alit&eacute;, le programme s'appelle maintenant tout simplement 'intranet'. Ceci dit, il faut maintenir un <a href="http://en.wikipedia.org/wiki/Symbolic_link">lien symbolique</a> de backend.php vers l'executable 'intranet', 
pour maintenir la compatibilit&eacute; avec les logiciels de changement de prix et Excel.
<ul>
<li><b>cron.php action=ogc_transfert</b>: Retransfert de mani&egrave;re int&eacute;grale les bases de donn&eacute;es importantes d'OGC (inv1, inv2, prix1, pritel, rev1, rev2) sur le serveur MySQL local.</li>
<li><b>cron.php action=ogc_ventes</b>: Rapport de ventes IBM 'on-line' quotidien (&agrave; la fermeture du magasin). Destinataires: info, Renelle. </li>
<li><b>cron.php action=ogc_gratteux</b>: Rapport de ventes IBM 'on-line' sur demande (&agrave; ressortir sur des grosses journ&eacute;e de ventes, tel les gratteux). Destinataires: Info, Renelle, Jean. </li>
</ul>
<p>Aussi voir <a href="backend.php">backend.php</a>.</p>
			`
			rona.HttpWriteResponse(response)
			rona.FlushHttp(0)
		}
	}
	if rona.IsShell() {
		//Dans les variables d'environnement, nous avons la variable $HOME qui nous indique que le programme est lancé via un shell
		//Donc, s'exécuter en tant que 'cron' (voir la methode Cron dans le package Rona.)
		rona.Cron()
	}
}
