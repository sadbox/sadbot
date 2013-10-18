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
)

var (
	config                  Config
	urlRegex, regexErr      = regexp.Compile(`(?i)\b((?:https?://|www\d{0,3}[.]|[a-z0-9.\-]+[.][a-z]{2,4}/)(?:[^\s()<>]+|\(([^\s()<>]+|(\([^\s()<>]+\)))*\))+(?:\(([^\s()<>]+|(\([^\s()<>]+\)))*\)|[^\s` + "`" + `!()\[\]{};:'".,<>?«»“”‘’]))`)
	httpRegex, httpRegexErr = regexp.Compile(`http(s)?://.*`)
)

const flickrApiUrl = "http://api.flickr.com/services/rest/"

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
	rand.Seed(time.Now().Unix())
	return rand.Intn(limit)
}

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
				postedUrl = parsedurl.Host
			}
			title = "Title: " + html.UnescapeString(title) + " (at " + postedUrl + ")"
			log.Println(title)
			conn.Privmsg(channel, title)
		}
	}
}

func dance(channel string, conn *irc.Conn) {
	conn.Privmsg(channel, ":D-<")
	time.Sleep(500 * time.Millisecond)
	conn.Privmsg(channel, ":D|<")
	time.Sleep(500 * time.Millisecond)
	conn.Privmsg(channel, ":D/<")
}

func haata(channel string, conn *irc.Conn) {
	flickrUrl, err := url.Parse(flickrApiUrl)
	if err != nil {
		log.Println(err)
		return
	}
	v := flickrUrl.Query()
	v.Set("method", "flickr.collections.getTree")
	v.Set("api_key", config.FlickrAPIKey)
	v.Set("user_id", "57321699@N06")
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

func handleMessage(conn *irc.Conn, line *irc.Line) {
	urllist := []string{}
	numlinks := 0

	// Special commands
	switch strings.TrimSpace(strings.Split(line.Args[1], " ")[0]) {
	case "!dance":
		go dance(line.Args[0], conn)
	case "!audio":
		if line.Nick == "sadbox" {
			conn.Privmsg(line.Args[0], "https://sadbox.org/static/audiophile.html")
		}
	case "!cst":
		if line.Nick == "sadbox" {
			conn.Privmsg(line.Args[0], "13,8#CSTMASTERRACE")
		}
	case "!haata":
		go haata(line.Args[0], conn)
	case "!search":
		go googSearch(line.Args[0], line.Args[1], conn)
	}

	// Commands that are read in from the config file
	for _, command := range config.Commands {
		if strings.HasPrefix(line.Args[1], command.Name) {
			conn.Privmsg(line.Args[0], command.Text)
		}
	}

NextWord:
	for _, word := range strings.Split(line.Args[1], " ") {
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
			go sendUrl(line.Args[0], word, conn)
		}

	}
	db, err := sql.Open("mysql", config.DBConn)
	if err != nil {
		log.Println(err)
	}
	defer db.Close()
	_, err = db.Exec("insert into messages (Nick, Ident, Host, Src, Cmd, Channel, Message, Time) values (?, ?, ?, ?, ?, ?, ?, ?)", line.Nick, line.Ident, line.Host, line.Src, line.Cmd, line.Args[0], line.Args[1], line.Time)
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
}

func main() {
	c := irc.SimpleClient(config.Nick, config.Ident, config.FullName)

	c.AddHandler(irc.CONNECTED,
		func(conn *irc.Conn, line *irc.Line) {
			conn.Join(config.Channel)
			conn.Privmsg("nickserv", "identify "+config.Nick+" "+config.IRCPass)
			log.Println("Connected!")
		})

	quit := make(chan bool)

	c.AddHandler(irc.DISCONNECTED,
		func(conn *irc.Conn, line *irc.Line) { quit <- true })

	c.AddHandler("PRIVMSG", handleMessage)

	if err := c.Connect("irc.freenode.net"); err != nil {
		log.Fatalln("Connection error: %s\n", err)
	}

	<-quit
}
