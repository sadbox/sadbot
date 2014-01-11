package main

import (
	irc "github.com/fluffle/goirc/client"
	"sync"
	"time"
)

var meebcast = meebCast{status: false}

type meebCast struct {
	mutex   sync.RWMutex
	turnOff *time.Timer
	status  bool
}

func meeba(channel, nick, command string, conn *irc.Conn) {
	if nick == "meeba" || nick == "sadbox" {
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
		go conn.Privmsg(channel, "Drinking Problem show is \u00030,3on air\u0003! Tune in: http://radio.abstractionpoint.org")
	} else {
		go conn.Privmsg(channel, "Drinking Problem show is \u00030,4off the air\u0003! Tune in: http://radio.abstractionpoint.org")
	}
}
