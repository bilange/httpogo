package rona

import (
    "encoding/xml"
    //"os"
    "fmt"
    "bytes"
    "strings"
)

//Utilisé en XML pour retourner l'information sur un produit.
type ProductInfo struct {
    XMLName     xml.Name            `xml:"root"`
    Rona        string              `xml:"rona"`
    Titre       string              `xml:"titre"`
    Desc1       string              `xml:"desc1"`
    Desc2       string              `xml:"desc2"`
    CodeProduit string              `xml:"codeproduit"`
    Style       string              `xml:"style"`
    Composante  string              `xml:"composante"`
    Technique   string              `xml:"technique"`
    Marque      string              `xml:"marque"`
    Cout        float64             `xml:"cout"`

    Pritel          PritelArray        `xml:"pritel,omitempty"`
    Localisations   LocalisationArray  `xml:"localisations"`
    UPC             UPCArray           `xml:"upcs"`
}

// ********** PRITEL **********

type PritelArray struct {
    Pritel   []Pritel  `xml:"prix,omitempty"`
}

type Pritel struct {
    XMLName         xml.Name    `xml:"prix"`
    Date            string      `xml:"date"`
    Datefin         string      `xml:"datefin"`
    Circulaire      string      `xml:"circulaire"`
    UV              string      `xml:"uv"`
    Prix            float64     `xml:"prix"`
    PrixCode        string      `xml:"prixcode"`
}

//Rajoute un element Pritel dans l'array ProductInfo.PritelArray.
func (p *ProductInfo) AddPritel(ptel Pritel) {
    p.Pritel.Pritel = append(p.Pritel.Pritel, ptel)
}

// ********** LOCALISATIONS **********

type LocalisationArray struct {
    Localisations   []Localisation  `xml:"localisations"`
}

type Localisation struct {
    XMLName         xml.Name    `xml:"localisation"`
    Localisation    string      `xml:"loc,attr"`
    Type            string      `xml:"type,attr"`
    Etiquette       int         `xml:"etiquette,attr"`
}

//Rajoute un element Localisation dans l'array ProductInfo.Localisations.
func (l *ProductInfo) AddLocalisation(loc string, t string, f int) {
    newLoc := Localisation{Localisation: loc, Type: t, Etiquette: f}
    l.Localisations.Localisations = append(l.Localisations.Localisations, newLoc)
}


// ********** CODES UPC **********

type UPCArray struct {
    UPCs   []UPC  `xml:"upcs"`
}

type UPC struct {
    XMLName         xml.Name    `xml:"upc"`
    Code            string      `xml:"code,attr"`
    UV              string      `xml:"uv,attr"`
    PrixCode        string      `xml:"prixcode,attr"`
    Format          string      `xml:"format,attr"`
    Prix            float64     `xml:"prix,attr"`
    Conversion      float64     `xml:"conversion,attr"`
}

//Rajoute un element UPC dans l'array ProductInfo.UPC.
func (p *ProductInfo) AddUPC(upc UPC) {
    p.UPC.UPCs = append(p.UPC.UPCs, upc)
}



//Cette fonction est appelé en cas de désastre dans une de nos requêtes MySQL, ou dans tout autre erreurs non-recouvrables pendant la construction de la réponse du serveur. Sera renvoyé au client un XML d'erreur expliquant ce qu'il s'est passé, et la connexion sera coupée d'avec le client.
func XMLDie(error string) {
    HttpContentType = "text/xml"
    HttpSetHeader("Expires: Thu, 01 Dec 1994 16:00:00 GMT\n")
    HttpWriteHeader("Server: Golang standalone CGI script\n")

    HttpWriteResponse(fmt.Sprintf("<root>\n\t<erreur>%s</erreur>\n\t<error>\n\t\t<no>-1</no>\n\t\t<text>%s</text>\n\t</error>\n</root>", error, error))
    FlushHttp(1)
}

//Cette fonction s'assure que des caractères spéciaux ne se faufile pas dans nos XML de réponse au client. Dans certains rares cas, OGC peut stocker (bien malgré lui et le fait qu'il ne serait pas compatible (du moins, les formulaires Informix ne les gèrent pas)) les caractères accentués, qui ne sont pas valide au format "brut" dans un XML. Il faut les "escaper" avec un entity numérique, genre "&#00120;" pour un é. Cette fonction reçoit un string a clarifier, et en ressort (via un return) en format avec entities.
func XMLSafeEntities(str string) string {
    //Debug:
    //str += ""
    //for _,v := range str {
        //fmt.Printf("%c", v)
    //}
    //return str

    var buf bytes.Buffer

    for _, v := range str {
        switch {
            case v == 38: 
                buf.WriteString("&amp;") //devrait etre fait via Marshall.
            case v == 60: 
                buf.WriteString("&lt;") //devrait etre fait via Marshall.
            case v == 62: 
                buf.WriteString("&gt;") //devrait etre fait via Marshall.
            case v >= 32 && v <= 126: 
                buf.WriteString(fmt.Sprintf("%c",v))
            case v >= 169 && v <= 255: //TEST accents
                //buf.WriteString(fmt.Sprintf("\\U%X",v))
                buf.WriteByte(byte(int(v)))
            default:
                //buf.WriteString(fmt.Sprintf("&#%00005d;", v)) //BUG: Go parse pour moi l'accent, mais ne l'escape pas en XML entities. Rajouter cette ligne va forcer "&" a etre un "&amp;"
                buf.WriteString(" ") 
        }
    }
    return buf.String()
}

//Apparamment, récupérer un string directement de MySQL est pris tel quel en valeur UTF8. Les caractères accentués peuvent causer problèmes. Ce hack corrige les valeurs strings qu'on veut extraire de MySQL (supposant qu'ils sont encodés ISO-8859-1) en décomposant le []byte, castant chacun des octets en char, et en recombinant le tout dans un string natif golang. Voir le source code pour plus d'information sur la technique et le pourquoi. 
func OGCStringConvert(b []byte) string {
    newstring := ""
    for _, v := range b {
        //fmt.Printf("Character: %c -- Decimal %d\n", int(v), v )
        newstring += (fmt.Sprintf("%c", v))
    }
    return newstring

    //string de base pour le code rona 3414069: "PLANC.HDF CHÊNE 8MM 25.8PC". 
    //Informix SQL ne contient aucune information sur comment les données sont encodées. 
    //On suppose ASCII, pour la simple raison qu'on ne peut pas voir correctement 
    //les informations entrées avec des accents, ainsi que les menus *ET* formulaires
    //OGC n'ont JAMAIS d'accents.
    //Lors de l'importation de Informix SQL à MySQL, MySQL a importé les infos avec
    //l'encodage ISO8859-1, le hexdump suivant le confirme ("CA" = "Ê")
    //mysql> select HEX(in1_desc_f) from inv1 where in1_code="3414069";
    //+------------------------------------------------------+
    //| HEX(in1_desc_f)                                      |
    //+------------------------------------------------------+
    //| 504C414E432E484446204348CA4E4520384D4D2032352E385043 |
    //+------------------------------------------------------+
    //
    //Ceci etant dit, Go l'importerait (de facon normale) en tant que: 
    //"PLANC.HDF CH�NE 8MM 25.8PC", dans le cas ou on fait un cast du string mysql en string go.
    //             ^--- Ce caractere est reconverti en valeur UTF-8 \uFFFD ('invalid') lors du cast en string go.
    //
    //OGCStringConvert() recoit un []byte et fait une boucle dans l'array. Pour chaque octet, un cast en tant que 
    //char est effectué, ce qui semble transformer notre string ISO8859-1 en UTF8 natif Go.

    //select in1_desc_f,HEX(in1_desc_f) from inv1 where in1_code="3414069";
}

//Transforme le formattage de la chaîne pour que tout les mots soient en majuscule sur la première lettre seulement.
func ToUpperWords(s string) string {
    mots := strings.Split(strings.ToLower(s), " ")
    rtn := ""
    for i, mot := range mots {
        if len(mot) == 0 { 
            rtn += " "
        } else if len(mot) > 1 { 
            rtn += fmt.Sprintf("%s%s",strings.ToUpper(string(mot[0])),mot[1:]) 
        } else {
            rtn += strings.ToUpper(string(mot[0]))
        }
        if i < len(mots)-1 { rtn += " " }
    }
    return rtn
}

