package banter

import (
	"net/http"
	"html/template"
	"time"
)

var templates = template.Must(template.ParseGlob("static/templates/*"))

type Article struct {
	Headline string
	Subhead string
	Twitter string
	Body string
	Date time.Time
	Coinmail string
	BTC uint64
	Id uint
}

var test_articles = []Article{
	{"Headline 1", "Subhead 1", "TwitterHand", "This is the body",time.Now(),"thedude@flailfast.com",100,1},
	{"Headline 2", "Subhead 2", "TwitterHand", "This is also a body",time.Now(),"thedude@flailfast.com",1000000,2} }

func init() {
	http.HandleFunc("/",indexHandler)
	http.HandleFunc("/about/",aboutHandler)
	http.HandleFunc("/article/",articleHandler)
	http.HandleFunc("/submit/",submitHandler)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "index.html", test_articles)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func aboutHandler(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "about.html", test_articles)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func articleHandler(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "article.html", test_articles[0])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "submit.html", test_articles)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
