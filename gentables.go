package main

import (
	"fmt"
	"log"
)

const baseTable = `CREATE TABLE Words (
    Nick VARCHAR(32),
    %s
    primary KEY (Nick));
    `

func updateWords(nick, message string) error {
	for word, regex := range badWords {
		numwords := len(regex.FindAllString(strings.ToLower(message), -1))
		if numwords == 0 {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		_, err = tx.Exec(`SELECT * FROM Words WHERE Nick=? FOR UPDATE`, nick)
		if err != nil {
			return err
		}
		_, err = tx.Exec(fmt.Sprintf(`INSERT INTO Words (Nick, %[1]s) VALUES (?, ?) ON DUPLICATE KEY UPDATE %[1]s=%[1]s+VALUES(%[1]s)`, word), nick, numwords)
		if err != nil {
			return err
		}
		err = tx.Commit()
		if err != nil {
			return err
		}
	}
	return nil
}

func genTables() {
	log.Println("Regenerating Words table")
	wordList := ""
	for _, word := range config.BadWords {
		wordList += word.Word + " INT(32) NOT NULL DEFAULT 0, "
	}

	_, err := db.Exec(`DROP TABLE IF EXISTS Words`)
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec(fmt.Sprintf(baseTable, wordList))
	if err != nil {
		log.Fatal(err)
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
		err = updateWords(nick, message)
		if err != nil {
			log.Fatal(err)
		}
	}
	if err = rows.Err(); err != nil {
		log.Fatal(err)
	}
	log.Println("Finished generating Words!")
}
