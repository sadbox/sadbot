// Copyright 2014 James McGuire. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	irc "github.com/fluffle/goirc/client"
	"net/url"
	"strings"
	"time"
)

// http://bash.org/?4281
func dance(channel string, conn *irc.Conn) {
	conn.Privmsg(channel, "\u0001ACTION dances :D-<\u0001")
	time.Sleep(500 * time.Millisecond)
	conn.Privmsg(channel, "\u0001ACTION dances :D|<\u0001")
	time.Sleep(500 * time.Millisecond)
	conn.Privmsg(channel, "\u0001ACTION dances :D/<\u0001")
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
	v.Set("btnI", "1")
	searchUrl.RawQuery = v.Encode()
	conn.Privmsg(channel, searchUrl.String())
}
