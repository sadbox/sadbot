package main

import (
	"database/sql"
	"encoding/xml"
	"flag"
	irc "github.com/fluffle/goirc/client"
	_ "github.com/go-sql-driver/mysql"
	"github.com/tv42/base58"
	"html"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var (
	config             Config
	urlRegex, regexErr = regexp.Compile(`(?i)\b((?:https?://|www\d{0,3}[.]|[` +
		`a-z0-9.\-]+[.][a-z]{2,4}/)(?:[^\s()<>]+|\(([^\s()<>]+|(\([^\s()<>]+` +
		`\)))*\))+(?:\(([^\s()<>]+|(\([^\s()<>]+\)))*\)|[^\s` + "`" + `!()\[` +
		`\]{};:'".,<>?«»“”‘’]))`)
	httpRegex, httpRegexErr = regexp.Compile(`http(s)?://.*`)
	markovkeys              []string
	markovmap               = make(map[string][]string)
)

const (
	flickrApiUrl = "http://api.flickr.com/services/rest/"
	PUNCTUATION  = `!"#$%&\'()*+,-./:;<=>?@[\\]^_{|}~` + "`"
)

type Config struct {
	Channel      string
	DBConn       string
	Nick         string
	Ident        string
	FullName     string
	FlickrAPIKey string
	IRCPass      string
	Commands     []Command `xml:">Command"`
}

type Command struct {
	Name string
	Text string
}

// The following four structs are all for the flickr api
type Setresp struct {
	Sets []Set `xml:"collections>collection>set"`
}

type Set struct {
	Id          string `xml:"id,attr"`
	Title       string `xml:"title,attr"`
	Description string `xml:"description,attr"`
}

type Photoresp struct {
	Photos []Photo `xml:"photoset>photo"`
}

type Photo struct {
	Id        int64  `xml:"id,attr"`
	Secret    string `xml:"secret,attr"`
	Server    string `xml:"server,attr"`
	Farm      string `xml:"farm,attr"`
	Title     string `xml:"title,attr"`
	Isprimary string `xml:"isprimary,attr"`
}

func cleanspaces(message string) []string {
	splitmessage := strings.Split(message, " ")
	var newslice []string
	for _, word := range splitmessage {
		if strings.TrimSpace(word) != "" {
			newslice = append(newslice, strings.Trim(strings.TrimSpace(word), PUNCTUATION))
		}
	}
	return newslice
}

// Command!
func markov(channel string, conn *irc.Conn) {
	var markovchain string
	messageLength := random(50) + 10
	for i := 0; i < messageLength; i++ {
		splitchain := strings.Split(markovchain, " ")
		if len(splitchain) < 2 {
			s := []rune(markovkeys[random(len(markovkeys))])
			s[0] = unicode.ToUpper(s[0])
			markovchain = string(s)
			continue
		}
		chainlength := len(splitchain)
		searchfor := strings.ToLower(splitchain[chainlength-2] + " " + splitchain[chainlength-1])
		if len(markovmap[searchfor]) == 0 || strings.LastIndex(markovchain, ".") < len(markovchain)-50 {
			s := []rune(markovkeys[random(len(markovkeys))])
			s[0] = unicode.ToUpper(s[0])
			markovchain = markovchain + ". " + string(s)
			continue
		}
		randnext := random(len(markovmap[searchfor]))
		markovchain = markovchain + " " + markovmap[searchfor][randnext]
	}
	conn.Privmsg(channel, markovchain+".")
}

// Build the whole markov chain.. this sits in memory, so adjust the limit and junk
func makeMarkov() error {
	db, err := sql.Open("mysql", "irclogger:irclogger@/irclogs")
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT Message from messages where Channel = '#geekhack' limit 30000`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var message string
		if err := rows.Scan(&message); err != nil {
			return err
		}
		message = strings.ToLower(message)
		newslice := cleanspaces(message)
		splitlength := len(newslice)
		for position, word := range newslice {
			if splitlength-2 <= position {
				break
			}
			wordkey := word + " " + newslice[position+1]
			markovmap[wordkey] = append(markovmap[wordkey], newslice[position+2])
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for key, _ := range markovmap {
		markovkeys = append(markovkeys, key)
	}
	return nil
}

// Just grab the page, don't care much about errors
func htmlfetch(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	respbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return respbody, nil
}

func random(limit int) int {
	rand.Seed(time.Now().UTC().UnixNano())
	return rand.Intn(limit)
}

// Try and grab the title for any URL's posted in the channel
func sendUrl(channel, postedUrl string, conn *irc.Conn) {
	log.Println("Fetching title for " + postedUrl + " In channel " + channel)
	if !httpRegex.MatchString(postedUrl) {
		postedUrl = "http://" + postedUrl
	}

	resp, err := http.Get(postedUrl)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()
	// This is necessary because if you do an ioutil.ReadAll() it will
	// block until the entire thing is read... which could be painful
	buf := make([]byte, 1024)
	respbody := []byte{}
	for i := 0; i < 30; i++ {
		n, err := resp.Body.Read(buf)
		if err != nil && err != io.EOF {
			return
		}
		if n == 0 {
			break
		}
		respbody = append(respbody, buf[:n]...)
	}

	stringbody := string(respbody)
	titlestart := strings.Index(stringbody, "<title>")
	titleend := strings.Index(stringbody, "</title>")
	if titlestart != -1 && titlestart != -1 {
		title := string(respbody[titlestart+7 : titleend])
		title = strings.TrimSpace(title)
		if title != "" {
			parsedurl, err := url.Parse(postedUrl)
			if err == nil {
				// This should only be the google.com in google.com/search&q=blah
				postedUrl = parsedurl.Host
			}
			// Example:
			// Title: sadbox . org (at sadbox.org)
			title = "Title: " + html.UnescapeString(title) + " (at " + postedUrl + ")"
			log.Println(title)
			conn.Privmsg(channel, title)
		}
	}
}

// http://bash.org/?4281
func dance(channel string, conn *irc.Conn) {
	conn.Privmsg(channel, "\u0001ACTION dances :D-<")
	time.Sleep(500 * time.Millisecond)
	conn.Privmsg(channel, "\u0001ACTION dances :D|<")
	time.Sleep(500 * time.Millisecond)
	conn.Privmsg(channel, "\u0001ACTION dances :D/<")
}

// Fetch a random picture from one of Haata's keyboard sets
func haata(channel string, conn *irc.Conn) {
	flickrUrl, err := url.Parse(flickrApiUrl)
	if err != nil {
		log.Println(err)
		return
	}
	v := flickrUrl.Query()
	v.Set("method", "flickr.collections.getTree")
	v.Set("api_key", config.FlickrAPIKey)
	// triplehaata's user_id
	v.Set("user_id", "57321699@N06")
	// Only the keyboard pics
	v.Set("collection_id", "57276377-72157635417889224")
	flickrUrl.RawQuery = v.Encode()

	sets, err := htmlfetch(flickrUrl.String())
	if err != nil {
		log.Println(err)
		return
	}
	var setresp Setresp
	xml.Unmarshal(sets, &setresp)
	randsetindex := random(len(setresp.Sets))
	randset := setresp.Sets[randsetindex].Id

	flickrUrl, err = url.Parse(flickrApiUrl)
	if err != nil {
		log.Println(err)
		return
	}
	v = flickrUrl.Query()
	v.Set("method", "flickr.photosets.getPhotos")
	v.Set("api_key", config.FlickrAPIKey)
	v.Set("photoset_id", randset)
	flickrUrl.RawQuery = v.Encode()

	pics, err := htmlfetch(flickrUrl.String())
	if err != nil {
		log.Println(err)
		return
	}
	var photoresp Photoresp
	xml.Unmarshal(pics, &photoresp)
	randpic := random(len(photoresp.Photos))
	// flickr's short url's are encoded using base58... this seems messy
	// Maybe use the proper long url?
	photostring := string(base58.EncodeBig([]byte{}, big.NewInt(photoresp.Photos[randpic].Id)))
	conn.Privmsg(channel, strings.TrimSpace(setresp.Sets[randsetindex].Title)+`: http://flic.kr/p/`+photostring)
}

func googSearch(channel, query string, conn *irc.Conn) {
	query = strings.TrimSpace(query[7:])
	if query == "" {
		conn.Privmsg(channel, "Example: !search stuff and junk")
		return
	}
	searchUrl, err := url.Parse("https://google.com/search")
	if err != nil {
		panic(err)
	}
	v := searchUrl.Query()
	v.Set("q", query)
	searchUrl.RawQuery = v.Encode()
	conn.Privmsg(channel, searchUrl.String())
}

// This function does all the dispatching for various commands
// as well as logging each message to the database
func handleMessage(conn *irc.Conn, line *irc.Line) {
	urllist := []string{}
	numlinks := 0

	// This is so that the bot can properly respond to pm's
	var channel string
	if conn.Me.Nick == line.Args[0] {
		channel = line.Nick
	} else {
		channel = line.Args[0]
	}
	message := line.Args[1]
	splitmessage := strings.Split(message, " ")

	// Special commands
	switch strings.TrimSpace(splitmessage[0]) {
	case "!dance":
		if line.Nick == "sadbox" {
			go dance(channel, conn)
		}
	case "!audio":
		if line.Nick == "sadbox" {
			conn.Privmsg(channel, "https://sadbox.org/static/audiophile.html")
		}
	case "!cst":
		if line.Nick == "sadbox" {
			conn.Privmsg(channel, "\u00039,13#CSTMASTERRACE")
		}
	case "!haata":
		go haata(channel, conn)
	case "!search":
		go googSearch(channel, message, conn)
	case "!chatter":
		go markov(channel, conn)
	}

	// Commands that are read in from the config file
	for _, command := range config.Commands {
		if strings.TrimSpace(splitmessage[0]) == command.Name {
			conn.Privmsg(channel, command.Text)
		}
	}

NextWord:
	for _, word := range splitmessage {
		word = strings.TrimSpace(word)
		if urlRegex.MatchString(word) {
			for _, subUrl := range urllist {
				if subUrl == word {
					continue NextWord
				}
			}
			numlinks++
			if numlinks > 3 {
				break
			}
			urllist = append(urllist, word)
			go sendUrl(channel, word, conn)
		}
	}

	db, err := sql.Open("mysql", config.DBConn)
	if err != nil {
		log.Println(err)
	}
	defer db.Close()
	_, err = db.Exec("insert into messages (Nick, Ident, Host, Src, Cmd, Channel,"+
		" Message, Time) values (?, ?, ?, ?, ?, ?, ?, ?)", line.Nick, line.Ident,
		line.Host, line.Src, line.Cmd, channel, message, line.Time)
	if err != nil {
		log.Println(err)
	}
}

func init() {
	log.Println("Starting sadbot")

	flag.Parse()

	if regexErr != nil {
		log.Panic(regexErr)
	}
	if httpRegexErr != nil {
		log.Panic(httpRegexErr)
	}

	xmlFile, err := ioutil.ReadFile("config.xml")
	if err != nil {
		log.Fatal(err)
	}
	xml.Unmarshal(xmlFile, &config)

	log.Println("Loaded config file!")
	log.Printf("Joining channel %s", config.Channel)
	log.Printf("Nick: %s", config.Nick)
	log.Printf("Ident: %s", config.Ident)
	log.Printf("FullName: %s", config.FullName)

	log.Printf("Found %d commands", len(config.Commands))
	for index, command := range config.Commands {
		log.Printf("%d %s: %s", index+1, command.Name, command.Text)
	}

	log.Println("Loading markov data.")
	err = makeMarkov()
	if err != nil {
		log.Panic(err)
	}
}

func main() {
	c := irc.SimpleClient(config.Nick, config.Ident, config.FullName)

	c.SSL = true

	c.AddHandler(irc.CONNECTED,
		func(conn *irc.Conn, line *irc.Line) {
			conn.Join(config.Channel)
			log.Println("Connected!")
		})

	quit := make(chan bool)

	c.AddHandler(irc.DISCONNECTED,
		func(conn *irc.Conn, line *irc.Line) { quit <- true })

	c.AddHandler("PRIVMSG", handleMessage)

	if err := c.Connect("irc.freenode.net", config.Nick+":"+config.IRCPass); err != nil {
		log.Fatalln("Connection error: %s\n", err)
	}

	<-quit
}
