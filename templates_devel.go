// +build devel

package main

import (
	"io/ioutil"
	"log"
)

func status_html() []byte {
	data, err := ioutil.ReadFile("status.html")
	if err != nil {
		log.Println("Could not open status.html", err)
		return []byte{}
	}
	return data
}
