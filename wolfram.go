package main

import (
	"encoding/xml"
	irc "github.com/fluffle/goirc/client"
	"log"
	"net/url"
	"strings"
)

const (
	wolframAPIUrl = `http://api.wolframalpha.com/v2/query`
	WHITESPACE    = "\t\n\u000b\u000c\r"
)

// Wolfram|Alpha structs
type Wolfstruct struct {
	Success bool  `xml:"success,attr"`
	Pods    []Pod `xml:"pod"`
}

type Pod struct {
	Title   string `xml:"title,attr"`
	Text    string `xml:"subpod>plaintext"`
	Primary bool   `xml:"primary,attr"`
}

func wolfram(channel, query string, conn *irc.Conn) {
	query = strings.TrimSpace(query[4:])
	if strings.TrimSpace(query) == "" {
		conn.Privmsg(channel, "Example: !ask pi")
		return
	}
	log.Printf("Searching wolfram alpha for %s", query)
	wolf, err := url.Parse(wolframAPIUrl)
	if err != nil {
		log.Println(err)
		return
	}
	v := wolf.Query()
	v.Set("input", query)
	v.Set("appid", config.WolframAPIKey)
	wolf.RawQuery = v.Encode()
	respbody, err := htmlfetch(wolf.String())
	if err != nil {
		log.Println(err)
		return
	}
	var wolfstruct Wolfstruct
	xml.Unmarshal(respbody, &wolfstruct)
	log.Println(wolfstruct)
	if wolfstruct.Success {
		for _, pod := range wolfstruct.Pods {
			if pod.Primary {
				log.Println(query)
				queryslice := []byte(query + ": " + pod.Title + " " + pod.Text)
				if len(queryslice) > 506 {
					query = string(queryslice[:507]) + "..."
				} else {
					query = string(queryslice)
				}
				conn.Privmsg(channel, removeChars(query, WHITESPACE))
				// Sometimes it returns multiple primary pods
				return
			}
		}
	}
	// If I couldn't find anything just give up...
	conn.Privmsg(channel, "I have no idea.")
}
