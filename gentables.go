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

const baseTable = `CREATE TABLE %[1]s (
    Nick VARCHAR(32),
    %[2]s
    primary KEY (Nick)) ENGINE=InnoDB DEFAULT CHARSET=utf8;
    `

func updateWords(channel, nick, message string, checkChannel bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	if checkChannel {
		for _, goodChannel := range config.Channels {
			if channel == goodChannel {
				break
			}
			// We won't update the badwords on channels that we don't track.
			return nil
		}
	}

	for word, regex := range badWords {
		numwords := len(regex.FindAllStringIndex(strings.ToLower(message), -1))
		if numwords == 0 {
			continue
		}
		formattedQuery := fmt.Sprintf("INSERT INTO `%[1]s_words` (Nick, %[2]s) VALUES (?, ?)"+
			" ON DUPLICATE KEY UPDATE %[2]s=%[2]s+VALUES(%[2]s)", channel, word)
		_, err = tx.Exec(formattedQuery, nick, numwords)
		if err != nil {
			tx.Rollback()
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
	log.Println("Regenerating Words tables")
	wordList := ""
	for _, word := range config.BadWords {
		wordList += word.Word + " INT(32) NOT NULL DEFAULT 0, "
	}

	_, err := db.Exec("DROP TABLE IF EXISTS `Channels`")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE Channels (
        Channel VARCHAR(32),
        primary KEY (Channel)) ENGINE=InnoDB DEFAULT CHARSET=utf8;`)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Creating channel tables")
	for _, channel := range config.Channels {
		_, err := db.Exec("INSERT INTO `Channels` (Channel) VALUES (?)", channel)
		if err != nil {
			log.Fatal(err)
		}

		_, err = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s_words`", channel))
		if err != nil {
			log.Fatal(err)
		}
		_, err = db.Exec(fmt.Sprintf(baseTable, fmt.Sprintf("`%s_words`", channel), wordList))
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Println("Finished creating channel tables")

	genChan := make(chan struct {
		Channel string
		Nick    string
		Message string
	})
	wg := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		go func() {
			for post := range genChan {
				err := updateWords(post.Channel, post.Nick, post.Message, false)
				if err != nil {
					log.Fatal(err)
				}
				wg.Done()
			}
		}()
	}

	log.Println("Starting word load")
	rows, err := db.Query(`SELECT messages.Channel, Nick, Message from messages right join Channels ON messages.Channel = Channels.Channel;`)
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var channel, nick, message string
		err := rows.Scan(&channel, &nick, &message)
		if err != nil {
			log.Fatal(err)
		}
		wg.Add(1)
		genChan <- struct {
			Channel string
			Nick    string
			Message string
		}{channel, nick, message}
	}

	wg.Wait()
	close(genChan)
	log.Println("Finished word load")

	if err = rows.Err(); err != nil {
		log.Fatal(err)
	}
	log.Println("Finished generating Words!")
}
