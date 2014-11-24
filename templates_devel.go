// +build devel

package main

import (
	"io/ioutil"
	"log"
)

func templates_status_html() ([]byte, error) {
	data, err := ioutil.ReadFile("templates/status.html")
	if err != nil {
		log.Println("Could not open status.html", err)
		return nil, err
	}
	return data, nil
}
