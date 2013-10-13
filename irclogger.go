package main

import (
    irc "github.com/fluffle/goirc/client"
    "log"
    "database/sql"
    "strings"
    "net/http"
    "io/ioutil"
    "os"
    "flag"
    "encoding/xml"
    _ "github.com/go-sql-driver/mysql"
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
        title := string(respbody[titlestart+7:titleend])
        title = "Title: " + title
        conn.Privmsg(channel, title)
    }
}

func handleMessage(conn *irc.Conn, line *irc.Line) {
    for _, word := range strings.Split(line.Args[1], " ") {
        word = strings.TrimSpace(word)
        if strings.HasPrefix(word, "http") {
            go sendUrl(line.Args[0], word, conn)
        }

    }
    db, err := sql.Open("mysql", config.DBConn)
    if err != nil {
        //log.Error(err)
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
    log.Println(config.Channel, config.DBConn, config.BotName)
    logfile, err := os.OpenFile("/var/log/sadbot", os.O_RDWR|os.O_APPEND, 0660)
    if err != nil {
        log.Fatal(err)
    }
    log.SetOutput(logfile)
    log.Printf("Joining channel %s", config.Channel)
    c := irc.SimpleClient(config.BotName)
    c.AddHandler(irc.CONNECTED,
        func(conn *irc.Conn, line *irc.Line) { conn.Join(config.Channel)
        log.Println("Connected!")})
    quit := make(chan bool)
    c.AddHandler(irc.DISCONNECTED,
        func(conn *irc.Conn, line *irc.Line) { quit <- true })
    c.AddHandler("PRIVMSG", handleMessage)

    if err := c.Connect("irc.freenode.net"); err != nil {
        log.Fatalln("Connection error: %s\n", err)
    }

    <-quit
}
