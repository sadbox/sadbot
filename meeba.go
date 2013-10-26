package main

import (
    irc "github.com/fluffle/goirc/client"
    "sync"
)

var meebcast = meebCast{status: false}

type meebCast struct {
    status bool
    mutex sync.RWMutex
}

func meeba(channel, nick, command string, conn *irc.Conn) {
    if nick == "meeba" || nick == "sadbox" {
        meebcast.mutex.Lock()
        if command == "on" {
            meebcast.status = true
        } else if command == "off" {
            meebcast.status = false
        }
        meebcast.mutex.Unlock()
    }
    meebcast.mutex.RLock()
    defer meebcast.mutex.RUnlock()
    if meebcast.status {
        go conn.Privmsg(channel, "Drinking Problem show is \u00030,3on air\u0003! Tune in: http://radio.abstractionpoint.org")
    } else {
        go conn.Privmsg(channel, "Drinking Problem show is \u00030,4off the air\u0003! Tune in: http://radio.abstractionpoint.org")
    }
}
