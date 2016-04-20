// Copyright 2014 James McGuire. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"strings"
	"sync"
	"time"

	irc "github.com/fluffle/goirc/client"
)

var meebcast = meebCast{status: false}

type meebCast struct {
	mutex   sync.RWMutex
	turnOff *time.Timer
	status  bool
}

func meeba(conn *irc.Conn, line *irc.Line) {
	if !strings.HasPrefix(line.Text(), "!meebcast") {
		return
	}
	splitline := strings.Split(line.Text(), " ")
	command := ""
	if len(splitline) > 1 {
		command = splitline[1]
	}
	if line.Nick == "meeba" || line.Nick == "sadbox" {
		if command == "on" {
			meebcast.mutex.Lock()
			meebcast.status = true

			meebcast.turnOff = time.AfterFunc(3*time.Hour, func() {
				meebcast.mutex.Lock()
				meebcast.status = false
				meebcast.mutex.Unlock()
			})

			meebcast.mutex.Unlock()
		} else if command == "off" {
			meebcast.mutex.Lock()
			meebcast.status = false
			if meebcast.turnOff != nil {
				meebcast.turnOff.Stop()
			}
			meebcast.mutex.Unlock()
		}
	}
	meebcast.mutex.RLock()
	defer meebcast.mutex.RUnlock()
	if meebcast.status {
		go conn.Privmsg(line.Target(), "The meebcats show is \u00030,3on air\u000f! Tune in: http://funkatize.me:8001/stream")
	} else {
		go conn.Privmsg(line.Target(), "The meebcats show is \u00030,4off the air\u000f! Tune in: http://funkatize.me:8001/stream")
	}
}
