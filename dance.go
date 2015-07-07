// Copyright 2014 James McGuire. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"time"

	irc "github.com/fluffle/goirc/client"
)

// http://bash.org/?4281
func dance(conn *irc.Conn, line *irc.Line) {
	if line.Nick != "sadbox" || getCommand(line) != "!dance" {
		return
	}
	conn.Privmsg(line.Target(), "\u0001ACTION dances :D-<\u0001")
	time.Sleep(500 * time.Millisecond)
	conn.Privmsg(line.Target(), "\u0001ACTION dances :D|<\u0001")
	time.Sleep(500 * time.Millisecond)
	conn.Privmsg(line.Target(), "\u0001ACTION dances :D/<\u0001")
}
