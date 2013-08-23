package banter

import (
	"appengine"
	"appengine/datastore"
	"bytes"
	"errors"
	"github.com/extemporalgenome/slug"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"time"
	//"banter/kekeke"
)

var funcMap = template.FuncMap{
	"dateFormat": time.Time.Format,
	"formatB":    FormatBTC,
	"isTop":      IsTop,
}

var templates = template.Must(template.New("").Funcs(funcMap).ParseGlob("static/templates/*"))
var artValidator = regexp.MustCompile(`^[0-9a-z-]+$`)
var emailValidator = regexp.MustCompile(`(^$|[-0-9a-zA-Z.+_]+@[-0-9a-zA-Z.+_]+\.[a-zA-Z]{2,4})`)

func FormatBTC(b int64) string {
	switch {
	case b == 0:
		return "0 BTC"
	case b < 100:
		return strconv.FormatInt(b,10) + "s"
	case b < 100000 && b >= 100:
		return strconv.FormatFloat(float64(b)/100, 'f', -1, 64) + " μBTC"
	case b >= 100000:
		return strconv.FormatFloat(float64(b/100)/1000, 'f', -1, 64) + " mBTC"
	default:
		return "???"
	}
}

func IsTop(a string) bool {
	if a == "top" {
		return true
	} else {
		return false
	}
}

type Article struct {
	Headline string
	Subhead  string
	Twitter  string
	Body     [][]byte
	Date     time.Time
	Coinmail string
	Coincode string
	BTC      int64
	SlugId   string
}

type ArtHead struct {
	Headline string
	Date int64
	BTC int64
	Coincode string
	SlugId string
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
	path := r.URL.Path[1:]
	show_alert := ""
	header_link := ""
	q := datastore.NewQuery("Article").Project("Headline","Date","BTC","Coincode","SlugId")
	switch {
	case path == "":
		q = q.Order("-BTC")
		header_link = "new"
	case path == "top":
		q = q.Order("-BTC")
		show_alert = "top"
		header_link = "new"
	case path == "new":
		q = q.Order("-Date")
		show_alert = "new"
		header_link = "top"
	default:
		http.NotFound(w, r)
	}
	var articles []ArtHead
	if _, err := q.GetAll(c, &articles); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err := templates.ExecuteTemplate(w, "index.html",struct{Arts []ArtHead; ShowAlert string; HeaderLink string}{articles,show_alert,header_link})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func aboutHandler(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "about.html", "no data needed")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func articleHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	var the_art Article
	article_path := r.URL.Path[5:]
	if !artValidator.MatchString(article_path) {
		http.NotFound(w, r)
		return
	}
	k := datastore.NewKey(c, "Article", article_path, 0, nil)
	if err := datastore.Get(c, k, &the_art); err == datastore.ErrNoSuchEntity {
		http.NotFound(w, r)
		return
	}
	err := templates.ExecuteTemplate(w, "article.html", the_art)
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
		new_bod := bytes.Split([]byte(f("bod")), []byte{'\r', '\n'})
		err = insertArticle(&Article{f("headline"), f("subhead"), f("twit"), new_bod, time.Now(), f("btc_add"), "fake_code", 0, ""}, r)
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
	i := 1
	for {
		k := datastore.NewKey(c, "Article", new_slug, 0, nil)
		if err := datastore.Get(c, k, &test_art); err == datastore.ErrNoSuchEntity {
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

/* func tweetEr() {
	//send out the tweets!
} */
