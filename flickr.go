// Copyright 2014 James McGuire. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"encoding/xml"
	irc "github.com/fluffle/goirc/client"
	"github.com/tv42/base58"
	"log"
	"math/big"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
)

const flickrApiUrl = "https://api.flickr.com/services/rest/"

type Setresp struct {
	Sets []Set `xml:"collections>collection>set"`
}

type Set struct {
	Id          string `xml:"id,attr"`
	Title       string `xml:"title,attr"`
	Description string `xml:"description,attr"`
}

type Photoresp struct {
	Photos []Photo `xml:"photoset>photo"`
}

type Photo struct {
	Id        int64  `xml:"id,attr"`
	Secret    string `xml:"secret,attr"`
	Server    string `xml:"server,attr"`
	Farm      string `xml:"farm,attr"`
	Title     string `xml:"title,attr"`
	Isprimary string `xml:"isprimary,attr"`
}

// Fetch a random picture from one of Haata's keyboard sets
func haata(channel string, conn *irc.Conn) {
	flickrUrl, err := url.Parse(flickrApiUrl)
	if err != nil {
		log.Println(err)
		return
	}
	v := flickrUrl.Query()
	v.Set("method", "flickr.collections.getTree")
	v.Set("api_key", config.FlickrAPIKey)
	// triplehaata's user_id
	v.Set("user_id", "57321699@N06")
	// Only the keyboard pics
	v.Set("collection_id", "57276377-72157635417889224")
	flickrUrl.RawQuery = v.Encode()

	sets, err := http.Get(flickrUrl.String())
	if err != nil {
		log.Println(err)
		return
	}
	defer sets.Body.Close()
	var setresp Setresp
	err = xml.NewDecoder(sets.Body).Decode(&setresp)
	if err != nil {
		log.Println(err)
		return
	}
	randsetindex := rand.Intn(len(setresp.Sets))
	randset := setresp.Sets[randsetindex].Id

	flickrUrl, err = url.Parse(flickrApiUrl)
	if err != nil {
		log.Println(err)
		return
	}
	v = flickrUrl.Query()
	v.Set("method", "flickr.photosets.getPhotos")
	v.Set("api_key", config.FlickrAPIKey)
	v.Set("photoset_id", randset)
	flickrUrl.RawQuery = v.Encode()

	pics, err := http.Get(flickrUrl.String())
	if err != nil {
		log.Println(err)
		return
	}
	defer pics.Body.Close()
	var photoresp Photoresp
	err = xml.NewDecoder(pics.Body).Decode(&photoresp)
	if err != nil {
		log.Println(err)
		return
	}
	randpic := rand.Intn(len(photoresp.Photos))
	// flickr's short url's are encoded using base58... this seems messy
	// Maybe use the proper long url?
	photostring := string(base58.EncodeBig([]byte{}, big.NewInt(photoresp.Photos[randpic].Id)))
	conn.Privmsg(channel, strings.TrimSpace(setresp.Sets[randsetindex].Title)+`: http://flic.kr/p/`+photostring)
}
