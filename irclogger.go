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
	"regexp"
	"strings"
	"time"
)

type Config struct {
	Channel      string
	DBConn       string
	Nick         string
	Ident        string
	FullName     string
	FlickrAPIKey string
	IRCPass      string
}

var (
	config             Config
	urlRegex, regexErr = regexp.Compile(`(?i)\b((?:https?://|www\d{0,3}[.]|[a-z0-9.\-]+[.][a-z]{2,4}/)(?:[^\s()<>]+|\(([^\s()<>]+|(\([^\s()<>]+\)))*\))+(?:\(([^\s()<>]+|(\([^\s()<>]+\)))*\)|[^\s` + "`" + `!()\[\]{};:'".,<>?«»“”‘’]))`)
	helpstring         = "Available commands are !hacking, !help, and !haata"
)

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

func sendUrl(channel, url string, conn *irc.Conn) {
	log.Println("Fetching title for " + url + " In channel " + channel)
	if !strings.HasPrefix(url, "http://") {
		if !strings.HasPrefix(url, "https://") {
			url = "http://" + url
		}
	}
	resp, err := http.Get(url)
	if err != nil {
		log.Println(err)
		return
	}
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
			if len(url) > 30 {
				url = string([]byte(url)[:30]) + "..."
			}
			title = "Title: " + html.UnescapeString(title) + " (" + url + ")"
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
	sets, err := htmlfetch(`http://api.flickr.com/services/rest/?method=flickr.collections.getTree&api_key=` + config.FlickrAPIKey + `&user_id=57321699@N06&collection_id=57276377-72157635417889224`)
	if err != nil {
		return
	}
	var setresp Setresp
	xml.Unmarshal(sets, &setresp)
	randsetindex := random(len(setresp.Sets))
	randset := setresp.Sets[randsetindex].Id

	pics, err := htmlfetch(`http://api.flickr.com/services/rest/?method=flickr.photosets.getPhotos&api_key=` + config.FlickrAPIKey + `&photoset_id=` + randset)
	if err != nil {
		return
	}
	var photoresp Photoresp
	xml.Unmarshal(pics, &photoresp)
	randpic := random(len(photoresp.Photos))
	returnbytes := []byte{}
	photostring := string(base58.EncodeBig(returnbytes, big.NewInt(photoresp.Photos[randpic].Id)))
	conn.Privmsg(channel, strings.TrimSpace(setresp.Sets[randsetindex].Title)+`: http://flic.kr/p/`+photostring)
}

func handleMessage(conn *irc.Conn, line *irc.Line) {
	urllist := []string{}
	numlinks := 0

	if strings.HasPrefix(line.Args[1], "!dance") && line.Nick == "sadbox" {
		go dance(line.Args[0], conn)
	} else if strings.HasPrefix(line.Args[1], "!audio") && line.Nick == "sadbox" {
		conn.Privmsg(line.Args[0], "https://sadbox.org/static/audiophile.html")
	} else if strings.HasPrefix(line.Args[1], "!hacking") {
		conn.Privmsg(line.Args[0], "This channel is about keyboards (not hacking), please read the topic.")
	} else if strings.HasPrefix(line.Args[1], "!help") {
		conn.Privmsg(line.Args[0], helpstring)
	} else if strings.HasPrefix(line.Args[1], "!haata") {
		go haata(line.Args[0], conn)
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

	xmlFile, err := ioutil.ReadFile("config.xml")
	if err != nil {
		log.Fatal(err)
	}
	xml.Unmarshal(xmlFile, &config)
}

func main() {
	log.Printf("Joining channel %s", config.Channel)
	log.Printf("Nick: %s Ident: %s FullName: %s", config.Nick, config.Ident, config.FullName)

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
