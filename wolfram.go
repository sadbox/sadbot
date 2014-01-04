package main

import (
	"encoding/xml"
	"fmt"
	irc "github.com/fluffle/goirc/client"
	"log"
	"net/url"
	"strings"
)

const wolframAPIUrl = `http://api.wolframalpha.com/v2/query`

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

func wolfram(channel, query, nick string, conn *irc.Conn) {
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
				response := strings.Split(pod.Title+": "+pod.Text, "\n")
				var numlines int
				if len(response) > 3 {
					numlines = 3
				} else {
					numlines = len(response)
				}
				query = fmt.Sprintf("(In reponse to: <%s> %s)", nick, query)
				if numlines == 1 {
					conn.Privmsg(channel, response[0]+" "+query)
				} else {
					for _, message := range response[:numlines] {
						conn.Privmsg(channel, message)
					}
					conn.Privmsg(channel, query)
				}
				// Sometimes it returns multiple primary pods
				return
			}
		}
	}
	// If I couldn't find anything just give up...
	conn.Privmsg(channel, "I have no idea.")
}
