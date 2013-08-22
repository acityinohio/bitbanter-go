package banter

import (
	"html/template"
	"net/http"
	"regexp"
	"time"
)

var funcMap = template.FuncMap{
	"dateFormat": time.Time.Format,
}

var templates = template.Must(template.New("").Funcs(funcMap).ParseGlob("static/templates/*"))
var artValidator = regexp.MustCompile(`^[1-9][0-9]*$`)
var emailValidator = regexp.MustCompile(`(^$|[-0-9a-zA-Z.+_]+@[-0-9a-zA-Z.+_]+\.[a-zA-Z]{2,4})`)

type Article struct {
	Headline string
	Subhead  string
	Twitter  string
	Body     string
	Date     time.Time
	Coinmail string
	BTC      float32
	Id       uint
}

var test_articles = []Article{
	{"Headline 1", "Subhead 1", "TwitterHand", "This is the body", time.Now(), "thedude@flailfast.com", 100, 1},
	{"Headline 2", "Subhead 2", "TwitterHand", "This is also a body", time.Now(), "thedude@flailfast.com", 1000000, 2},
}

func init() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/about/", aboutHandler)
	http.HandleFunc("/art/", articleHandler)
	http.HandleFunc("/submit/", submitHandler)
	http.HandleFunc("/submit/new/", newHandler)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "index.html", test_articles)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func aboutHandler(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "about.html", test_articles)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func articleHandler(w http.ResponseWriter, r *http.Request) {
	article := r.URL.Path[5:]
	if !artValidator.MatchString(article) {
		http.NotFound(w, r)
		return
	}
	err := templates.ExecuteTemplate(w, "article.html", test_articles[0])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "submit.html", r.FormValue("err"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func newHandler(w http.ResponseWriter, r *http.Request) {
	f := r.FormValue
	test := Article{f("headline"), f("subhead"), f("twit"), f("bod"), time.Now(), f("btc_add"), float32(0), uint(3)}
	err := ""
	switch {
	case len(test.Headline) < 10:
		err = "Your headline is too short!"
	case len(test.Headline) > 110:
		err = "Your headline is too long!"
	case len(test.Subhead) > 110:
		err = "Your subhead is too long!"
	case len(test.Body) > 15000:
		err = "Your body is too long!"
	case !emailValidator.MatchString(test.Coinmail):
		err = "Not a valid email...leave blank to forgo your tips if you'd prefer"
	default:
		test_articles = append(test_articles, test)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/submit/?err="+err, http.StatusFound)
	return
}
