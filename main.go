// Copyright 2014 James McGuire. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	//"flag"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	irc "github.com/fluffle/goirc/client"
	//"github.com/fluffle/goirc/logging/glog"
	_ "github.com/go-sql-driver/mysql"
	"github.com/mvdan/xurls"
	"golang.org/x/net/html/charset"
)

var (
	config         Config
	httpRegex      = regexp.MustCompile(`https?://.*`)
	findWhiteSpace = regexp.MustCompile(`\s+`)
	db             *sql.DB
	badWords       = make(map[string]*regexp.Regexp)
)

const FREENODE = "irc.freenode.net"

type Config struct {
	Channels             []string
	DBConn               string
	Nick                 string
	Ident                string
	FullName             string
	FlickrAPIKey         string
	WolframAPIKey        string
	OpenWeatherMapAPIKey string
	IRCPass              string
	RebuildWords         bool
	Commands             []struct {
		Channel  string
		Commands []struct {
			Name string
			Text string
		}
	}
	BadWords []struct {
		Word  string
		Query string
	}
}

func getCommand(line *irc.Line) string {
	splitmessage := strings.Split(line.Text(), " ")
	cmd := strings.TrimSpace(splitmessage[0])
	return cmd
}

// Try and grab the title for any URL's posted in the channel
func sendUrl(channel, unparsedURL string, conn *irc.Conn, nick string) {
	if !httpRegex.MatchString(unparsedURL) {
		unparsedURL = `http://` + unparsedURL
	}
	postedUrl, err := url.Parse(unparsedURL)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Fetching title for " + postedUrl.String() + " In channel " + channel)

	resp, err := http.Get(postedUrl.String())
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		log.Println("http server return error.")
		return
	}
	respbody := []byte{}
	if resp.Header.Get("Content-Type") == "" {
		buf := make([]byte, 512)
		bufsize, err := resp.Body.Read(buf)
		if err != nil {
			log.Println("adding content type failed")
		}
		resp.Header.Set("Content-Type", http.DetectContentType(buf[:bufsize]))
		respbody = append(respbody, buf[:bufsize]...)
	}

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		log.Println("content-type is not text/html")
		return
	}

	utf8Body, err := charset.NewReader(resp.Body, resp.Header.Get("Content-Type"))
	if err != nil {
		log.Println("Error converting page to utf8:", err)
		return
	}
	restofbody, err := ioutil.ReadAll(io.LimitReader(utf8Body, 50000))
	if err != nil {
		log.Println("Error reading posted link:", err)
		return
	}
	respbody = append(respbody, restofbody...)
	query, err := goquery.NewDocumentFromReader(bytes.NewReader(respbody))
	if err != nil {
		log.Println("Error parsing HTML tree:", err)
		return
	}
	title := query.Find("head").Find("title").First().Text()
	title = strings.TrimSpace(title)
	if len(title) == 0 || !utf8.ValidString(title) {
		return
	}
	// Example:
	// Title: sadbox . org (at sadbox.org)
	hostNick := fmt.Sprintf(" (%s)", postedUrl.Host)
	formattedTitle := html.UnescapeString(title)
	formattedTitle = findWhiteSpace.ReplaceAllString(formattedTitle, " ")
	if len(formattedTitle) > conn.Config().SplitLen-len(hostNick)-1 {
		formattedTitle = formattedTitle[:conn.Config().SplitLen-len(hostNick)-1]
	}
	formattedTitle = formattedTitle + hostNick
	log.Println(formattedTitle)
	conn.Privmsg(channel, formattedTitle)
}

func logMessage(conn *irc.Conn, line *irc.Line) {
	_, err := db.Exec("insert into messages (Nick, Ident, Host, Src, Cmd, Channel,"+
		" Message, Time) values (?, ?, ?, ?, ?, ?, ?, ?)", line.Nick, line.Ident,
		line.Host, line.Src, line.Cmd, line.Target(), line.Text(), line.Time)
	if err != nil {
		log.Println(err)
	}
	err = updateWords(line.Target(), line.Nick, line.Text(), true)
	if err != nil {
		log.Println(err)
	}
}

func checkForUrl(conn *irc.Conn, line *irc.Line) {
	if strings.HasPrefix(line.Text(), "#") {
		return
	}
	urllist := make(map[string]struct{})
	for _, item := range xurls.Relaxed.FindAllString(line.Text(), -1) {
		urllist[item] = struct{}{}
	}
	numlinks := 0
	for item, _ := range urllist {
		numlinks++
		if numlinks > 3 {
			break
		}
		go sendUrl(line.Target(), item, conn, line.Nick)
	}
}

func cst(conn *irc.Conn, line *irc.Line) {
	if line.Nick != "sadbox" || getCommand(line) != "!cst" {
		return
	}
	go conn.Privmsg(line.Target(), "\u00039,13#CSTMASTERRACE")
}

// Commands that are read in from the config file
func configCommands(conn *irc.Conn, line *irc.Line) {
	splitmessage := strings.Split(line.Text(), " ")
AllConfigs:
	for _, commandConfig := range config.Commands {
		if commandConfig.Channel == line.Target() || commandConfig.Channel == "default" {
			for _, command := range commandConfig.Commands {
				if getCommand(line) == command.Name {
					var response string
					if len(splitmessage) >= 2 {
						response = fmt.Sprintf("%s: %s", splitmessage[1], command.Text)
					} else {
						response = command.Text
					}
					go conn.Privmsg(line.Target(), response)
					break AllConfigs
				}
			}
		}
	}
}

func init() {
	log.Println("Starting sadbot")

	rand.Seed(time.Now().UTC().UnixNano())

	configfile, err := os.Open("config.json")
	if err != nil {
		log.Fatal(err)
	}
	err = json.NewDecoder(configfile).Decode(&config)
	if err != nil {
		log.Fatal(err)
	}

	for _, word := range config.BadWords {
		badWords[word.Word] = regexp.MustCompile(word.Query)
	}

	log.Println("Loaded config file!")
	log.Printf("Joining: %s", config.Channels)
	log.Printf("Nick: %s", config.Nick)
	log.Printf("Ident: %s", config.Ident)
	log.Printf("FullName: %s", config.FullName)

	numcommands := 0
	for _, commandConfig := range config.Commands {
		for _, command := range commandConfig.Commands {
			numcommands++
			log.Printf("%d %s/%s: %s", numcommands, commandConfig.Channel, command.Name, command.Text)
		}
	}
	log.Printf("Found %d commands", numcommands)
}

func main() {
	//flag.Parse()
	//glog.Init()
	var err error
	db, err = sql.Open("mysql", config.DBConn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxIdleConns(100)
	db.SetMaxOpenConns(200)

	go makeMarkov()

	buildchan := make(chan os.Signal, 1)
	signal.Notify(buildchan, syscall.SIGUSR1)
	go func() {
		for _ = range buildchan {
			genTables()
		}
	}()

	ircConfig := irc.NewConfig(config.Nick, config.Ident, config.FullName)
	ircConfig.SSL = true
	ircConfig.SSLConfig = &tls.Config{ServerName: FREENODE}
	ircConfig.Server = FREENODE
	ircConfig.Pass = config.Nick + ":" + config.IRCPass

	c := irc.Client(ircConfig)

	c.HandleFunc(irc.CONNECTED,
		func(conn *irc.Conn, line *irc.Line) {
			for _, channel := range config.Channels {
				log.Printf("Joining %s", channel)
				conn.Join(channel)
			}
			log.Println("Connected!")
		})
	quit := make(chan bool)

	c.HandleFunc(irc.DISCONNECTED,
		func(conn *irc.Conn, line *irc.Line) { print("disconnected!"); quit <- true })

	// Handle all the things
	c.HandleFunc(irc.PRIVMSG, logMessage)
	c.HandleFunc(irc.ACTION, logMessage)

	c.HandleFunc(irc.PRIVMSG, checkForUrl)
	c.HandleFunc(irc.ACTION, checkForUrl)

	c.HandleFunc(irc.PRIVMSG, haata)
	c.HandleFunc(irc.PRIVMSG, wolfram)
	c.HandleFunc(irc.PRIVMSG, meeba)
	c.HandleFunc(irc.PRIVMSG, markov)
	c.HandleFunc(irc.PRIVMSG, dance)
	c.HandleFunc(irc.PRIVMSG, cst)
	c.HandleFunc(irc.PRIVMSG, roll)
	c.HandleFunc(irc.PRIVMSG, btc)
	c.HandleFunc(irc.PRIVMSG, lastSeen)
	c.HandleFunc(irc.PRIVMSG, showWeather)
	c.HandleFunc(irc.PRIVMSG, showQuote)
	c.HandleFunc(irc.PRIVMSG, configCommands)

	if err := c.Connect(); err != nil {
		log.Fatalln("Connection error: %s\n", err)
	}

	<-quit
	log.Fatal("SHITTING THE FUCK DOWN")
}
