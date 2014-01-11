package main

import (
	"database/sql"
	"encoding/json"
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
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	config   Config
	urlRegex = regexp.MustCompile(`(?i)\b((?:https?://|www\d{0,3}[.]|[` +
		`a-z0-9.\-]+[.][a-z]{2,4}/)(?:[^\s()<>]+|\(([^\s()<>]+|(\([^\s()<>]+` +
		`\)))*\))+(?:\(([^\s()<>]+|(\([^\s()<>]+\)))*\)|[^\s` + "`" + `!()\[` +
		`\]{};:'".,<>?«»“”‘’]))`)
	httpRegex = regexp.MustCompile(`http(s)?://.*`)
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
	Commands      []struct {
		Name string
		Text string
	}
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
	if resp.StatusCode >= 400 {
		log.Println("http server return error.")
		return
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		log.Println("content-type is not text/html")
		return
	}
	respbody, err := ioutil.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		log.Println("error reading posted link")
		return
	}
	stringbody := string(respbody)
	titlestart := strings.Index(stringbody, "<title>")
	titleend := strings.Index(stringbody, "</title>")
	if titlestart != -1 && titlestart != -1 {
		title := string(respbody[titlestart+7 : titleend])
		title = strings.TrimSpace(title)
		if title != "" && utf8.ValidString(title) {
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
			conn.Privmsg(channel, "https://sadbox.org/static/stuff/audiophile.html")
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
		if line.Nick == "sadbox" {
			go markov(channel, conn)
		}
	case "!ask":
		go wolfram(channel, message, line.Nick, conn)
	case "!meebcast":
		var command string
		if len(splitmessage) < 2 {
			command = ""
		} else {
			command = strings.TrimSpace(splitmessage[1])
		}
		go meeba(channel, line.Nick, command, conn)
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

	configfile, err := os.Open("config.json")
	if err != nil {
		log.Fatal(err)
	}
	decoder := json.NewDecoder(configfile)
	err = decoder.Decode(&config)
	if err != nil {
		log.Fatal(err)
	}

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

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
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
	c.AddHandler("ACTION", handleMessage)

	if err := c.Connect("irc.freenode.net", config.Nick+":"+config.IRCPass); err != nil {
		log.Fatalln("Connection error: %s\n", err)
	}

	<-quit
}
