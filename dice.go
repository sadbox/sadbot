// Copyright 2014 James McGuire. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"strings"

	irc "github.com/fluffle/goirc/client"
	"github.com/justinian/dice"
)

func roll(conn *irc.Conn, line *irc.Line) {
	if !strings.HasPrefix(line.Text(), "!roll") {
		return
	}
	rolls := strings.TrimSpace(strings.TrimPrefix(line.Text(), "!roll"))
	allRolls := []string{}
	for _, diceroll := range strings.Split(rolls, " ") {
		if strings.TrimSpace(diceroll) == "" {
			continue
		}
		diceResult, err := dice.Roll(diceroll)
		if err != nil {
			result := fmt.Sprintf("%s: That doesn't look right... (%s)", line.Nick, diceroll)
			conn.Privmsg(line.Target(), result)
			log.Println("Error rolling dice:", err)
			return
		}
		allRolls = append(allRolls, fmt.Sprintf("%s: %s", diceroll, diceResult.String()))
	}
	message := strings.Join(allRolls, " \u00B7 ")
	if message != "" {
		message = line.Nick + ": " + message
		conn.Privmsg(line.Target(), message)
	}
}
