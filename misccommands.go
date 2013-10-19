package main

import (
	irc "github.com/fluffle/goirc/client"
	"net/url"
	"strings"
	"time"
)

// http://bash.org/?4281
func dance(channel string, conn *irc.Conn) {
	conn.Privmsg(channel, "\u0001ACTION dances :D-<")
	time.Sleep(500 * time.Millisecond)
	conn.Privmsg(channel, "\u0001ACTION dances :D|<")
	time.Sleep(500 * time.Millisecond)
	conn.Privmsg(channel, "\u0001ACTION dances :D/<")
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
