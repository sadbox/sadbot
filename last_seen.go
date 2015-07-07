// Copyright 2014 James McGuire. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"strings"

	irc "github.com/fluffle/goirc/client"
)

const lastSeenQuery = `select Time, Message from messages where ` +
	`channel = ? and Nick = ? order by Time desc limit 1`

func lastSeen(conn *irc.Conn, line *irc.Line) {
	if !strings.HasPrefix(line.Text(), "!last") {
		return
	}
	nick := strings.TrimSpace(strings.TrimPrefix(line.Text(), "!last"))
	rows, err := db.Query(lastSeenQuery, line.Target(), nick)
	if err != nil {
		log.Println("Error while preparing query for last message:", err)
	}
	var timestamp, message string
	for rows.Next() {
		if err := rows.Scan(&timestamp, &message); err != nil {
			log.Println("Error fetching from the db:", err)
		}
	}
	result := ""
	if timestamp != "" {
		result = fmt.Sprintf("%s: %s UTC <%s> %s", line.Nick, timestamp, nick, message)
	} else {
		result = fmt.Sprintf("%s: I haven't seen %s", line.Nick, nick)
	}
	conn.Privmsg(line.Target(), result)
}
