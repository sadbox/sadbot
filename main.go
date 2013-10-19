package main

import (
	"database/sql"
	"encoding/xml"
	"flag"
	irc "github.com/fluffle/goirc/client"
	_ "github.com/go-sql-driver/mysql"
	"html"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	config             Config
	urlRegex, regexErr = regexp.Compile(`(?i)\b((?:https?://|www\d{0,3}[.]|[` +
		`a-z0-9.\-]+[.][a-z]{2,4}/)(?:[^\s()<>]+|\(([^\s()<>]+|(\([^\s()<>]+` +
		`\)))*\))+(?:\(([^\s()<>]+|(\([^\s()<>]+\)))*\)|[^\s` + "`" + `!()\[` +
		`\]{};:'".,<>?«»“”‘’]))`)
	httpRegex, httpRegexErr = regexp.Compile(`http(s)?://.*`)
)

type Config struct {
	Channel       string
	DBConn        string
	Nick          string
	Ident         string
	FullName      string
	FlickrAPIKey  string
	WolframAPIKey string
	IRCPass       string
	Commands      []Command `xml:">Command"`
}

type Command struct {
	Name string
	Text string
}

// Used in markov and wolfram
func removeChars(bigstring, removeset string) string {
	for _, character := range removeset {
		bigstring = strings.Replace(bigstring, string(character), "", -1)
	}
	return bigstring
}

// Just grab the page, don't care much about errors
// Used in flickr and wolfram
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

func logMessage(line *irc.Line, channel, message string) {
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

func checkForUrl(channel string, splitmessage []string, conn *irc.Conn) {
	urllist := []string{}
	numlinks := 0
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
}

// This function does all the dispatching for various commands
// as well as logging each message to the database
func handleMessage(conn *irc.Conn, line *irc.Line) {
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
	case "!ask":
		go wolfram(channel, message, conn)
	}

	// Commands that are read in from the config file
	for _, command := range config.Commands {
		if strings.TrimSpace(splitmessage[0]) == command.Name {
			conn.Privmsg(channel, command.Text)
		}
	}

	// This is what looks at each word and tries to figure out if it's a URL
	go checkForUrl(channel, splitmessage, conn)

	// Shove that shit in the database!
	go logMessage(line, channel, message)
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
