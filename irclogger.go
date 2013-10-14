package main

import (
	"database/sql"
	"encoding/xml"
	"flag"
	irc "github.com/fluffle/goirc/client"
	_ "github.com/go-sql-driver/mysql"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
)

type Config struct {
	Channel string
	DBConn  string
	BotName string
}

var (
	config             Config
	urlRegex, regexErr = regexp.Compile(`(?i)\b((?:https?://|www\d{0,3}[.]|[a-z0-9.\-]+[.][a-z]{2,4}/)(?:[^\s()<>]+|\(([^\s()<>]+|(\([^\s()<>]+\)))*\))+(?:\(([^\s()<>]+|(\([^\s()<>]+\)))*\)|[^\s` + "`" + `!()\[\]{};:'".,<>?«»“”‘’]))`)
)

func sendUrl(channel, url string, conn *irc.Conn) {
	if !strings.HasPrefix(url, "http://") {
		url = "http://" + url
	}
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	respbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	stringbody := string(respbody)
	titlestart := strings.Index(stringbody, "<title>")
	titleend := strings.Index(stringbody, "</title>")
	if titlestart != -1 && titlestart != -1 {
		title := string(respbody[titlestart+7 : titleend])
		if strings.TrimSpace(title) != "" {
			title = "Title: " + html.UnescapeString(title) + " (" + url + ")"
			log.Println(title)
			conn.Privmsg(channel, strings.Replace(title, "\n", " ", -1))
		}
	}
}

func handleMessage(conn *irc.Conn, line *irc.Line) {
	urllist := []string{}
	numlinks := 0
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

	c := irc.SimpleClient(config.BotName)

	c.AddHandler(irc.CONNECTED,
		func(conn *irc.Conn, line *irc.Line) {
			conn.Join(config.Channel)
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
