package banter

import (
	"appengine"
	"appengine/datastore"
	"github.com/extemporalgenome/slug"
	"html/template"
	"net/http"
	"regexp"
	"time"
	"errors"
	"strconv"
	//"banter/kekeke"
)

var funcMap = template.FuncMap{
	"dateFormat": time.Time.Format,
}

var templates = template.Must(template.New("").Funcs(funcMap).ParseGlob("static/templates/*"))
var artValidator = regexp.MustCompile(`^[0-9a-z-]+$`)
var emailValidator = regexp.MustCompile(`(^$|[-0-9a-zA-Z.+_]+@[-0-9a-zA-Z.+_]+\.[a-zA-Z]{2,4})`)

type Article struct {
	Headline string
	Subhead  string
	Twitter  string
	Body     []byte
	Date     time.Time
	Coinmail string
	Coincode string
	BTC      int64
	SlugId   string
}

var test_articles = []Article{
	{"Headline 1", "Subhead 1", "TwitterHand", []byte("This is the body"), time.Now(), "thedude@flailfast.com", "fakecode1", 100, "headline-1"},
	{"Headline 2", "Subhead 2", "TwitterHand", []byte("This is also a body"), time.Now(), "thedude@flailfast.com", "fakecode2", 1000000, "headline-2"},
}

func init() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/about/", aboutHandler)
	http.HandleFunc("/art/", articleHandler)
	http.HandleFunc("/submit/", submitHandler)
	//http.HandlerFunc("/hey_listen/",btcHandler)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	q := datastore.NewQuery("Article").Order("-Date")
	var articles []Article
	if _, err := q.GetAll(c, &articles); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err := templates.ExecuteTemplate(w, "index.html", articles)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func aboutHandler(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "about.html", 0)
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
	var submit_msg error = nil
	if r.Method == "POST" {
		submit_msg = newArticleHandler(w, r)
	}
	err := templates.ExecuteTemplate(w, "submit.html", submit_msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func newArticleHandler(w http.ResponseWriter, r *http.Request) error {
	f := r.FormValue
	var err error = nil
	switch {
	case len(f("headline")) < 10:
		err = errors.New("Your headline is too short!")
	case len(f("headline")) > 110:
		err = errors.New("Your headline is too long!")
	case len(f("subhead")) > 110:
		err = errors.New("Your subhead is too long!")
	case len(f("bod")) < 200:
		err = errors.New("Your body is too short!")
	case len(f("bod")) > 15000:
		err = errors.New("Your body is too long!")
	case !emailValidator.MatchString(f("btc_add")):
		err = errors.New("Not a valid email...leave blank to forgo your tips if you'd prefer")
	default:
		err = insertArticle(&Article{f("headline"), f("subhead"), f("twit"), []byte(f("bod")), time.Now(), f("btc_add"), "fake_code", 0, ""},  r)
		if err != nil {
			return err
		}
		err = errors.New("Article submitted successfully!")
	}
	return err
}

func insertArticle(a *Article, r *http.Request) error {
	var test_art Article
	c := appengine.NewContext(r)
	slugger := slug.Slug(a.Headline)
	new_slug := slugger
	i := 1;
	for {
		k := datastore.NewKey(c, "Article", new_slug, 0, nil)
		if err:= datastore.Get(c, k, &test_art); err == datastore.ErrNoSuchEntity {
			a.SlugId = new_slug
			_, err := datastore.Put(c, k, a)
			if err != nil {
				return err
			}
			return nil
		}
		new_slug = slugger + `-` + strconv.Itoa(i)
		i++
	}
	return nil
}

/* func coinButtonCode (&Article) string {
	//generate new codes for coinbase buttons for each article
}*/

/* func btcHandler(w http.ResponseWriter, r *http.Request) {
	//disburse btc to authors, update btc totals
} */
