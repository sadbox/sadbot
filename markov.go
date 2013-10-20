package main

import (
	"database/sql"
	irc "github.com/fluffle/goirc/client"
	_ "github.com/go-sql-driver/mysql"
	"strings"
	"sync"
	"unicode"

	"log"
)

var (
	markovData Markov
)

const PUNCTUATION = `!"#$%&\'()*+,-./:;<=>?@[\\]^_{|}~` + "`"

type Markov struct {
	mutex  sync.RWMutex
	keys   []string
	bigmap map[string][]string
}

func (m *Markov) Init() {
	m.bigmap = make(map[string][]string)
}

func cleanspaces(message string) []string {
	splitmessage := strings.Split(message, " ")
	var newslice []string
	for _, word := range splitmessage {
		if strings.TrimSpace(word) != "" {
			newslice = append(newslice, removeChars(strings.TrimSpace(word), PUNCTUATION))
		}
	}
	return newslice
}

func removeChars(bigstring, removeset string) string {
	for _, character := range removeset {
		bigstring = strings.Replace(bigstring, string(character), "", -1)
	}
	return bigstring
}

// This is what generates the actual markov chain
func markov(channel string, conn *irc.Conn) {
	markovData.mutex.RLock()
	var markovchain string
	messageLength := random(50) + 10
	for i := 0; i < messageLength; i++ {
		splitchain := strings.Split(markovchain, " ")
		if len(splitchain) < 2 {
			s := []rune(markovData.keys[random(len(markovData.keys))])
			s[0] = unicode.ToUpper(s[0])
			markovchain = string(s)
			continue
		}
		chainlength := len(splitchain)
		searchfor := strings.ToLower(splitchain[chainlength-2] + " " + splitchain[chainlength-1])
		if len(markovData.bigmap[searchfor]) == 0 || strings.LastIndex(markovchain, ".") < len(markovchain)-50 {
			s := []rune(markovData.keys[random(len(markovData.keys))])
			s[0] = unicode.ToUpper(s[0])
			markovchain = markovchain + ". " + string(s)
			continue
		}
		randnext := random(len(markovData.bigmap[searchfor]))
		markovchain = markovchain + " " + markovData.bigmap[searchfor][randnext]
	}
	conn.Privmsg(channel, markovchain+".")
	markovData.mutex.RUnlock()
}

// Build the whole markov chain.. this sits in memory, so adjust the limit and junk
func makeMarkov() {
	db, err := sql.Open("mysql", config.DBConn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT Message from messages where Channel = '#geekhack' limit 30000`)
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var message string
		if err := rows.Scan(&message); err != nil {
			log.Fatal(err)
		}
		message = strings.ToLower(message)
		newslice := cleanspaces(message)
		splitlength := len(newslice)
		for position, word := range newslice {
			if splitlength-2 <= position {
				break
			}
			wordkey := word + " " + newslice[position+1]
			markovData.bigmap[wordkey] = append(markovData.bigmap[wordkey], newslice[position+2])
		}
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
	for key, _ := range markovData.bigmap {
		markovData.keys = append(markovData.keys, key)
	}
	markovData.mutex.Unlock()
}

func init() {
	log.Println("Loading markov data.")
	markovData.Init()
	markovData.mutex.Lock()
	go makeMarkov()
}
