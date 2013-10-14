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
	"strings"
)

type Config struct {
	Channel string
	DBConn  string
	BotName string
}

var config Config

func sendUrl(channel, url string, conn *irc.Conn) {
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
			title = "Title: " + html.UnescapeString(title)
			conn.Privmsg(channel, title)
		}
	}
}

func handleMessage(conn *irc.Conn, line *irc.Line) {
	urllist := []string{}
	numlinks := 0
NextWord:
	for _, word := range strings.Split(line.Args[1], " ") {
		word = strings.TrimSpace(word)
		if strings.HasPrefix(word, "http") {
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

func main() {
	flag.Parse()

	xmlFile, err := ioutil.ReadFile("config.xml")
	if err != nil {
		log.Fatal(err)
	}
	xml.Unmarshal(xmlFile, &config)

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
