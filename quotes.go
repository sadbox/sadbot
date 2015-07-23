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

const (
	updateQuoteQuery = `INSERT INTO quotes (nick, quote) VALUES (?, ?) ON DUPLICATE KEY UPDATE nick=?, quote=?;`
	quoteQuery       = `SELECT quote FROM quotes where nick=?;`
)

func findQuote(nick string) (string, error) {
	rows, err := db.Query(quoteQuery, nick)
	if err != nil {
		return "", err
	}
	var quote string
	for rows.Next() {
		if err := rows.Scan(&quote); err != nil {
			return "", err
		}
	}
	return quote, nil
}

func updateQuote(nick, quote string) error {
	_, err := db.Exec(updateQuoteQuery, nick, quote, nick, quote)
	if err != nil {
		return err
	}
	return nil
}

func showQuote(conn *irc.Conn, line *irc.Line) {
	if !strings.HasPrefix(line.Text(), "!quote") {
		return
	}
	message := strings.TrimSpace(strings.TrimPrefix(line.Text(), "!quote"))

	target_nick := line.Nick
	targeted := false

	switch {
	case strings.HasPrefix(message, "set "):
		message = strings.TrimSpace(strings.TrimPrefix(message, "set "))
		targeted = false
		if strings.HasPrefix(message, "@") {
			targeted = true
			split_message := strings.SplitN(message, " ", 2)
			if len(split_message) != 2 {
				result := fmt.Sprintf("%s: That doesn't look right...", line.Nick)
				conn.Privmsg(line.Target(), result)
				return
			}
			target_nick = strings.TrimPrefix(split_message[0], "@")
			message = split_message[1]
		}
		log.Printf("Updating quote for %s to %s", target_nick, message)
		err := updateQuote(target_nick, message)
		if err != nil {
			log.Println("Error updating quote:", err)
		}
		result := ""
		if targeted {
			result = fmt.Sprintf("%s: %s's quote has been updated", line.Nick, target_nick)
		} else {
			result = fmt.Sprintf("%s: Your quote has been updated", target_nick)
		}
		conn.Privmsg(line.Target(), result)
		return
	case strings.HasPrefix(message, "clear"):
		log.Printf("Clearing quote for %s", line.Nick)
		err := updateQuote(line.Nick, "")
		if err != nil {
			log.Println("Error updating quote:", err)
		}
		result := fmt.Sprintf("%s: Your quote has been cleared in the database.", line.Nick)
		conn.Privmsg(line.Target(), result)
		return
	case strings.HasPrefix(message, "help"):
		result := fmt.Sprintf("%s: Quotes! set will set your quote (!quote set dickbutt),"+
			" clear will remove your stored quote, \"!quote nick\" will show the quote for another nick (!quote sadbox),"+
			" and help will show this message.", line.Nick)
		conn.Privmsg(line.Target(), result)
		return
	}

	if message != "" {
		target_nick = message
		targeted = true
	}

	quote, err := findQuote(target_nick)
	if err != nil {
		log.Println("Error fetching quote from DB:", err)
		return
	}
	if quote == "" {
		result := ""
		if targeted {
			result = fmt.Sprintf("%s: %s hasn't ever set a quote.", line.Nick, target_nick)
		} else {
			result = fmt.Sprintf("%s: You need to specify a quote at least once. (!quote set dickbutt)", line.Nick)
		}
		conn.Privmsg(line.Target(), result)
		return
	}

	result := ""
	if targeted {
		result = fmt.Sprintf("%s: <%s> %s", line.Nick, target_nick, quote)
	} else {
		result = fmt.Sprintf("<%s> %s", line.Nick, quote)
	}
	conn.Privmsg(line.Target(), result)
}
