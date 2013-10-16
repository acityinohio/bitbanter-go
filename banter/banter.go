package banter

import (
	"appengine"
	"appengine/datastore"
	"appengine/memcache"
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

var templates = template.Must(template.New("").Funcs(funcMap).ParseGlob("templates/*"))
var artValidator = regexp.MustCompile(`^[0-9a-z-]+$`)
var emailValidator = regexp.MustCompile(`(^$|[-0-9a-zA-Z.+_]+@[-0-9a-zA-Z.+_]+\.[a-zA-Z]{2,4})`)

const maxDays = -30

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

type Tip struct {
	Id       string
	SlugId   string
	Headline string
	Date     time.Time
	Coinmail string
	BTC      int64
	Status   string
	Paid     bool
}

func init() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/about", aboutHandler)
	http.HandleFunc("/art/", articleHandler)
	http.HandleFunc("/submit", submitHandler)
	http.HandleFunc("/heylisten", btcHandler)
	http.HandleFunc("/heypayme", taskMaster)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	path := r.URL.Path[1:]
	q := datastore.NewQuery("Article")
	showAlert := ""
	headerLink := ""
	indexKey := "top"
	switch {
	case path == "":
		headerLink = "new"
		q = q.Order("-BTC").Order("-Date").Filter("Old =", false)
	case path == "top":
		showAlert = "top"
		headerLink = "new"
		q = q.Order("-BTC").Order("-Date").Filter("Old =", false)
	case path == "new":
		showAlert = "new"
		headerLink = "top"
		indexKey = "new"
		q = q.Order("-Date").Filter("Old =", false)
	default:
		http.NotFound(w, r)
		return
	}
	var articles []Article
	if item, err := memcache.Get(c, indexKey); err != nil {
		if _, err := q.GetAll(c, &articles); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		buf, _ := json.Marshal(articles)
		art := &memcache.Item{
			Key:   indexKey,
			Value: buf,
		}
		memcache.Set(c, art)
	} else {
		json.Unmarshal(item.Value, &articles)
	}
	err := templates.ExecuteTemplate(w, "index.html", struct {
		Arts       []Article
		ShowAlert  string
		HeaderLink string
	}{articles, showAlert, headerLink})
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
	var theArt Article
	articlePath := r.URL.Path[5:]
	if !artValidator.MatchString(articlePath) {
		http.NotFound(w, r)
		return
	}
	k := datastore.NewKey(c, "Article", articlePath, 0, nil)
	if item, err := memcache.Get(c, articlePath); err != nil {
		if err := datastore.Get(c, k, &theArt); err == datastore.ErrNoSuchEntity {
			http.NotFound(w, r)
			return
		}
		buf, _ := json.Marshal(theArt)
		art := &memcache.Item{
			Key:   articlePath,
			Value: buf,
		}
		memcache.Set(c, art)
	} else {
		json.Unmarshal(item.Value, &theArt)
	}
	if theArt.Date.Before(time.Now().AddDate(0, 0, maxDays)) {
		theArt.Old = true
		datastore.Put(c, k, &theArt)
		memcache.DeleteMulti(c, []string{"top", "new"})
	}
	err := templates.ExecuteTemplate(w, "article.html", theArt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	var submitMsg error = nil
	if r.Method == "POST" {
		submitMsg = newArticleHandler(w, r)
	}
	err := templates.ExecuteTemplate(w, "submit.html", submitMsg)
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
		newBod := bytes.Split([]byte(f("bod")), []byte{'\r', '\n', '\r', '\n'})
		err = insertArticle(&Article{f("headline"), f("subhead"), f("twit"), newBod, time.Now(), f("btc_add"), "", 0, false, ""}, r)
		if err != nil {
			return err
		}
		time.Sleep(2500 * time.Millisecond)
		http.Redirect(w, r, "/new", http.StatusFound)
	}
	return err
}

func insertArticle(a *Article, r *http.Request) error {
	var testArt Article
	var k *datastore.Key
	c := appengine.NewContext(r)
	slugger := slug.Slug(a.Headline)
	newSlug := slugger
	i := 1
	for {
		k = datastore.NewKey(c, "Article", newSlug, 0, nil)
		if err := datastore.Get(c, k, &testArt); err == datastore.ErrNoSuchEntity {
			a.SlugId = newSlug
			break
		}
		newSlug = slugger + `-` + strconv.Itoa(i)
		i++
	}
	coinCode, err := coinButtonCode(a.Headline, a.SlugId, c)
	if err != nil {
		return err
	}
	a.Coincode = coinCode
	if _, err := datastore.Put(c, k, a); err != nil {
		return err
	}
	buf, _ := json.Marshal(a)
	art := &memcache.Item{
		Key:   a.SlugId,
		Value: buf,
	}
	memcache.Set(c, art)
	memcache.DeleteMulti(c, []string{"top", "new"})
	go tweetEr(c, a.Headline, a.SlugId)
	return nil
}

func coinButtonCode(headline string, slug string, c appengine.Context) (string, error) {
	const coinButtonUrl = "https://coinbase.com/api/v1/buttons"
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

	req := CoinReq{coinButton{headline, "0.001", "BTC", "donation", "none",
		"You, being awesome, decided to support a bitbanter author for writing a good article. Go you!",
		slug, false, true}, kekeke.Da_Key,
	}

	b, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	buf := bytes.NewReader(b)

	client := urlfetch.Client(c)
	resp, err := client.Post(coinButtonUrl, "application/json", buf)
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
	client.HttpClient = urlfetch.Client(c)

	data := url.Values{}
	tweet := headline + " http://bitbanter.com/art/" + slugger
	data.Set("status", tweet)
	body := strings.NewReader(data.Encode())
	req, _ := http.NewRequest("POST", "/1.1/statuses/update.json", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client.SendRequest(req)
}

func btcHandler(w http.ResponseWriter, r *http.Request) {
	type CoinOrder struct {
		Order struct {
			Button          map[string]interface{}
			Created_at      string
			Custom          string
			Customer        map[string]interface{}
			Id              string
			Receive_address string
			Status          string
			Total_btc       struct {
				Cents        int64
				Currency_iso string
			}
			Total_native struct {
				Cents        int64
				Currency_iso string
			}
			Transaction struct {
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
	var (
		message CoinOrder
		k, l    *datastore.Key
		theArt  Article
		theTip  Tip
	)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&message); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	k = datastore.NewKey(c, "Article", message.Order.Custom, 0, nil)
	l = datastore.NewKey(c, "Tip", message.Order.Id, 0, nil)
	err := datastore.Get(c, k, &theArt)
	if err != nil && err == datastore.ErrNoSuchEntity {
		return
	}
	err = datastore.Get(c, l, &theTip)
	if err == nil {
		if message.Order.Status == "cancelled" {
			if theTip.Status == "cancelled" {
				return
			} else if theTip.Status == "completed" {
				theTip.Status = "cancelled"
				theTip.Paid = true
				datastore.Put(c, l, &theTip)
				theArt.BTC -= theTip.BTC
				datastore.Put(c, k, &theArt)
				buf, _ := json.Marshal(theArt)
				art := &memcache.Item{
					Key:   theArt.SlugId,
					Value: buf,
				}
				memcache.Set(c, art)
				return
			}
		} else {
			return
		}
	}
	tipTime, _ := time.Parse(time.RFC3339, message.Order.Created_at)
	theTip = Tip{message.Order.Id, theArt.SlugId, theArt.Headline, tipTime, theArt.Coinmail, message.Order.Total_btc.Cents, message.Order.Status, false}
	err = datastore.RunInTransaction(c, func(c appengine.Context) error {
		theArt.BTC += theTip.BTC
		_, err = datastore.Put(c, k, &theArt)
		buf, _ := json.Marshal(theArt)
		art := &memcache.Item{
			Key:   theArt.SlugId,
			Value: buf,
		}
		memcache.Set(c, art)
		_, err = datastore.Put(c, l, &theTip)
		return err
	}, &datastore.TransactionOptions{XG: true})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	memcache.DeleteMulti(c, []string{"top", "new"})
	w.WriteHeader(http.StatusOK)
	return
}

func taskMaster(w http.ResponseWriter, r *http.Request) {
	var allUnpaidTips []Tip
	c := appengine.NewContext(r)
	q := datastore.NewQuery("Tip").Filter("Paid =", false)

	_, err := q.GetAll(c, &allUnpaidTips)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, tip := range allUnpaidTips {
		if tip.Date.Before(time.Now().Add(-2 * time.Hour)) {
			k := datastore.NewKey(c, "Tip", tip.Id, 0, nil)
			if tip.Coinmail == "" {
				if err := sendMoney(kekeke.Da_Coinmail, tip.BTC, tip.Headline, c); err != nil {
					continue
				}
			} else {
				theirMoney := tip.BTC * 8 / 10
				myMoney := tip.BTC * 2 / 10
				if err := sendMoney(tip.Coinmail, theirMoney, tip.Headline, c); err != nil {
					continue
				}
				sendMoney(kekeke.Da_Coinmail, myMoney, tip.Headline, c)
			}
			tip.Paid = true
			datastore.Put(c, k, &tip)
		}
	}
	return
}

func sendMoney(email string, money int64, headline string, c appengine.Context) error {
	const coinTransferUrl = "https://coinbase.com/api/v1/transactions/send_money"
	type transInfo struct {
		To     string `json:"to"`
		Amount string `json:"amount"`
		Notes  string `json:"notes"`
	}
	type Transfer struct {
		Transaction transInfo `json:"transaction"`
		Api_key     string    `json:"api_key"`
	}

	moneyString := strconv.FormatFloat(float64(money)/float64(1e8), 'f', -1, 64)
	noteString := "You got a Bitbanter tip for \"" + headline + "\""
	req := Transfer{transInfo{email, moneyString, noteString}, kekeke.Da_Key}

	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	buf := bytes.NewReader(b)

	client := urlfetch.Client(c)
	_, err = client.Post(coinTransferUrl, "application/json", buf)
	if err != nil {
		return err
	}
	return nil
}
