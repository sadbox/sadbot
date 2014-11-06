// Copyright 2014 James McGuire. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

const baseTable = `CREATE TABLE words (
    Nick VARCHAR(32),
    %s
    primary KEY (Nick)) ENGINE=InnoDB DEFAULT CHARSET=utf8;
    `

func updateWords(nick, message string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for word, regex := range badWords {
		numwords := len(regex.FindAllStringIndex(strings.ToLower(message), -1))
		if numwords == 0 {
			continue
		}
		_, err = tx.Exec(fmt.Sprintf(`INSERT INTO words (Nick, %[1]s) VALUES (?, ?)`+
			` ON DUPLICATE KEY UPDATE %[1]s=%[1]s+VALUES(%[1]s)`, word), nick, numwords)
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func genTables() {
	log.Println("Regenerating Words table")
	wordList := ""
	for _, word := range config.BadWords {
		wordList += word.Word + " INT(32) NOT NULL DEFAULT 0, "
	}

	_, err := db.Exec(`DROP TABLE IF EXISTS words`)
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec(fmt.Sprintf(baseTable, wordList))
	if err != nil {
		log.Fatal(err)
	}

	genChan := make(chan struct {
		Nick    string
		Message string
	})
	wg := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		go func() {
			for post := range genChan {
				err := updateWords(post.Nick, post.Message)
				if err != nil {
					log.Fatal(err)
				}
				wg.Done()
			}
		}()
	}

	rows, err := db.Query(`SELECT Nick, Message from messages WHERE channel='#geekhack'`)
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var nick, message string
		err := rows.Scan(&nick, &message)
		if err != nil {
			log.Fatal(err)
		}
		wg.Add(1)
		genChan <- struct {
			Nick    string
			Message string
		}{nick, message}
	}

	wg.Wait()
	close(genChan)

	if err = rows.Err(); err != nil {
		log.Fatal(err)
	}
	log.Println("Finished generating Words!")
}
