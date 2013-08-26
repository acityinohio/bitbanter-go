package banter

import (
	"appengine"
	"appengine/datastore"
	"appengine/urlfetch"
	"banter/kekeke"
	"bytes"
	"encoding/json"
	"errors"
	"github.com/extemporalgenome/slug"
	"github.com/kurrik/oauth1a"
	"github.com/kurrik/twittergo"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var funcMap = template.FuncMap{
	"dateFormat": time.Time.Format,
	"formatB":    FormatBTC,
	"isTop":      IsTop,
}

var templates = template.Must(template.New("").Funcs(funcMap).ParseGlob("static/templates/*"))
var artValidator = regexp.MustCompile(`^[0-9a-z-]+$`)
var emailValidator = regexp.MustCompile(`(^$|[-0-9a-zA-Z.+_]+@[-0-9a-zA-Z.+_]+\.[a-zA-Z]{2,4})`)

const max_days = -30

func FormatBTC(b int64) string {
	switch {
	case b == 0:
		return "0 BTC"
	case b < 100:
		return strconv.FormatInt(b, 10) + "s"
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
	Old      bool
	SlugId   string
}

func init() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/about", aboutHandler)
	http.HandleFunc("/art/", articleHandler)
	http.HandleFunc("/submit", submitHandler)
	http.HandleFunc("/heylisten", btcHandler)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	path := r.URL.Path[1:]
	show_alert := ""
	header_link := ""
	q := datastore.NewQuery("Article")
	switch {
	case path == "":
		q = q.Order("-BTC").Order("-Date").Filter("Old =", false)
		header_link = "new"
	case path == "top":
		q = q.Order("-BTC").Order("-Date").Filter("Old =", false)
		show_alert = "top"
		header_link = "new"
	case path == "new":
		q = q.Order("-Date").Filter("Old =", false)
		show_alert = "new"
		header_link = "top"
	default:
		http.NotFound(w, r)
	}
	var articles []Article
	if _, err := q.GetAll(c, &articles); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err := templates.ExecuteTemplate(w, "index.html", struct {
		Arts       []Article
		ShowAlert  string
		HeaderLink string
	}{articles, show_alert, header_link})
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
	if the_art.Date.Before(time.Now().AddDate(0, 0, max_days)) {
		the_art.Old = true
		datastore.Put(c, k, &the_art)
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
		new_bod := bytes.Split([]byte(f("bod")), []byte{'\r', '\n', '\r', '\n'})
		err = insertArticle(&Article{f("headline"), f("subhead"), f("twit"), new_bod, time.Now(), f("btc_add"), "", 0, false, ""}, r)
		if err != nil {
			return err
		}
		time.Sleep(1000 * time.Millisecond)
		http.Redirect(w, r, "/new", http.StatusFound)
	}
	return err
}

func insertArticle(a *Article, r *http.Request) error {
	var test_art Article
	var k *datastore.Key
	c := appengine.NewContext(r)
	slugger := slug.Slug(a.Headline)
	new_slug := slugger
	i := 1
	for {
		k = datastore.NewKey(c, "Article", new_slug, 0, nil)
		if err := datastore.Get(c, k, &test_art); err == datastore.ErrNoSuchEntity {
			a.SlugId = new_slug
			break
		}
		new_slug = slugger + `-` + strconv.Itoa(i)
		i++
	}
	coin_code, err := coinButtonCode(a.Headline, a.SlugId, c)
	if err != nil {
		return err
	}
	a.Coincode = coin_code
	if _, err := datastore.Put(c, k, a); err != nil {
		return err
	}
	go tweetEr(c, a.Headline, a.SlugId)
	return nil
}

func coinButtonCode(headline string, slug string, c appengine.Context) (string, error) {
	const coin_button_url = "https://coinbase.com/api/v1/buttons"
	type coinButton struct {
		Name               string `json:"name"`
		Price_string       string `json:"price_string"`
		Price_currency_iso string `json:"price_currency_iso"`
		Type               string `json:"type"`
		Style              string `json:"style"`
		Description        string `json:"description"`
		Custom             string `json:"custom"`
		Variable_price     bool   `json:"variable_price"`
		Choose_price       bool   `json:"choose_price"`
	}
	type CoinReq struct {
		Button  coinButton `json:"button"`
		Api_key string     `json:"api_key"`
	}

	req := CoinReq{coinButton{headline, "0.01", "BTC", "donation", "none",
		"You, being awesome, decided to support a bitbanter author for writing a good article. Go you!",
		slug, false, true}, kekeke.Da_Key,
	}

	b, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	buf := bytes.NewReader(b)

	client := urlfetch.Client(c)
	resp, err := client.Post(coin_button_url, "application/json", buf)
	if err != nil {
		return "", err
	}

	dec := json.NewDecoder(resp.Body)
	var res map[string]interface{}
	var resolve map[string]interface{}
	if err := dec.Decode(&res); err != nil {
		return "", err
	}
	if res["success"].(bool) == false {
		return "", errors.New("Coinbase button generator failed")
	}
	resolve = res["button"].(map[string]interface{})
	return resolve["code"].(string), nil
}

func tweetEr(c appengine.Context, headline string, slugger string) {
	config := &oauth1a.ClientConfig{ConsumerKey: kekeke.Consumer_Key, ConsumerSecret: kekeke.Consumer_Secret}
	user := oauth1a.NewAuthorizedConfig(kekeke.Token, kekeke.Token_Secret)
	client := twittergo.NewClient(config, user)
	//for Appengine:
	client.HttpClient = urlfetch.Client(c)

	data := url.Values{}
	tweet := headline + " http://www.bitbanter.com/art/" + slugger
	data.Set("status", tweet)
	body := strings.NewReader(data.Encode())
	req, _ := http.NewRequest("POST", "/1.1/statuses/update.json", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client.SendRequest(req)
}

func btcHandler(w http.ResponseWriter, r *http.Request) {
	type Callback struct {
		Order struct {
			Id         string
			Created_at string
			Status     string
			Total_btc  struct {
				Cents        int64
				Currency_iso string
			}
			Total_native struct {
				Cents        int64
				Currency_iso string
			}
			Custom          string
			Receive_address string
			Button          map[string]interface{}
			Transaction     struct {
				Id            string
				Hash          string
				Confirmations int
			}
		}
	}
	if r.URL.RawQuery != kekeke.Da_Secret {
		http.Error(w, "not cool dude", http.StatusForbidden)
		return
	}
	c := appengine.NewContext(r)
	var message Callback
	var k *datastore.Key
	var theArt Article
	var theirMoney int64
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&message); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if message.Order.Status == "cancelled" {
		return
	}
	k = datastore.NewKey(c, "Article", message.Order.Custom, 0, nil)
	err := datastore.RunInTransaction(c, func(c appengine.Context) error {
		err := datastore.Get(c, k, &theArt)
		if err != nil && err == datastore.ErrNoSuchEntity {
			return err
		}
		theArt.BTC += message.Order.Total_btc.Cents
		_, err = datastore.Put(c, k, &theArt)
		return err
	}, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if theArt.Coinmail == "" {
		return
	}
	w.WriteHeader(http.StatusOK)
	go func() {
		time.Sleep(1 * time.Hour)
		theirMoney = message.Order.Total_btc.Cents * 8 / 10
		if err := sendMoney(theArt.Coinmail, theirMoney, theArt.Headline, c); err != nil {
			return
		}
	}()
	return
}

func sendMoney(email string, money int64, headline string, c appengine.Context) error {
	const coin_transfer_url = "https://coinbase.com/api/v1/transactions/send_money"
	type transInfo struct {
		To     string `json:"to"`
		Amount string `json:"amount"`
		Notes  string `json:"notes"`
	}
	type Transfer struct {
		Transaction transInfo `json:"transaction"`
		Api_key     string    `json:"api_key"`
	}

	money_string := strconv.FormatFloat(float64(money)/float64(1e8), 'f', -1, 64)
	note_string := "You got a Bitbanter tip for writing \"" + headline + "\""
	req := Transfer{transInfo{email, money_string, note_string}, kekeke.Da_Key}

	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	buf := bytes.NewReader(b)

	client := urlfetch.Client(c)
	_, err = client.Post(coin_transfer_url, "application/json", buf)
	if err != nil {
		return err
	}
	return nil
}
