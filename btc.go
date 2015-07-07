// Copyright 2014 James McGuire. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"text/template"

	irc "github.com/fluffle/goirc/client"
)

var btcTmpl = template.Must(template.New("btc").Parse(`{{.Nick}}: BTC:{{.Currency}} 15m {{.Symbol}}{{printf "%.2f" .Fifteen}}, Last {{.Symbol}}{{printf "%.2f" .Last}}, Buy {{.Symbol}}{{printf "%.2f" .Buy}}, Sell {{.Symbol}}{{printf "%.2f" .Sell}}`))

type Ticker map[string]struct {
	Fifteen        float64 `json:"15m"`
	Last           float64 `json:"last"`
	Buy            float64 `json:"buy"`
	Sell           float64 `json:"sell"`
	Symbol         string  `json:"symbol"`
	Nick, Currency string
}

func btc(conn *irc.Conn, line *irc.Line) {
	if !strings.HasPrefix(line.Text(), "!btc") {
		return
	}
	resp, err := http.Get("https://blockchain.info/ticker")
	if err != nil {
		log.Println("Couldn't get current info:", err)
		return
	}
	defer resp.Body.Close()
	var ticker Ticker
	err = json.NewDecoder(resp.Body).Decode(&ticker)
	if err != nil {
		fmt.Println("Error decoding BTC data:", err)
		return
	}

	splitline := strings.Split(strings.TrimSpace(line.Text()), " ")

	currencies := []string{}
	for k, _ := range ticker {
		currencies = append(currencies, k)
	}

	if len(splitline) == 1 {
		message := fmt.Sprintf("%s: Choose a currency! %s are available.", line.Nick, strings.Join(currencies, ", "))
		conn.Privmsg(line.Target(), message)
		return
	}

	currency := strings.TrimSpace(splitline[1])
	rates, ok := ticker[currency]
	if !ok {
		message := fmt.Sprintf("%s: I couldn't find any data on %s, please choose from %s.", line.Nick, currency, strings.Join(currencies, ", "))
		conn.Privmsg(line.Target(), message)
		return
	}

	rates.Nick = line.Nick
	rates.Currency = currency
	var b bytes.Buffer
	btcTmpl.Execute(&b, rates)
	conn.Privmsg(line.Target(), b.String())
}
