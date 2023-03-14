package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/sa6mwa/anystore"
)

// All fields need to be exported.
type MyConfig struct {
	ListenAddress string
	Username      string
	Token         string
	Endpoints     []*Endpoint
}

type Endpoint struct {
	ID   int
	Name string
	URL  string
}

func main() {
	defaultConf := &MyConfig{
		ListenAddress: "0.0.0.0:1234",
		Username:      "superuser",
		Token:         "abc123",
		Endpoints: []*Endpoint{
			{ID: 1, Name: "Endpoint 1", URL: "https://endpoint1.local"},
			{ID: 2, Name: "Endpoint 2", URL: "https://endpoint2.local"},
		},
	}
	file := "~/.myconfigfile.db"

	var configuration MyConfig

	if err := anystore.Unstash(&anystore.StashConfig{
		File:          file,
		EncryptionKey: anystore.DefaultEncryptionKey,
		Key:           "configuration",
		Thing:         &configuration,
	}, defaultConf); err != nil {
		log.Fatal(err)
	}

	if len(os.Args) > 1 && os.Args[1] == "edit" {
		if err := anystore.EditThing(&anystore.StashConfig{
			File:          file,
			EncryptionKey: anystore.DefaultEncryptionKey,
			Key:           "configuration",
			Thing:         &configuration,
			// Editor: "/usr/bin/emacs",
		}, defaultConf); err != nil {
			log.Fatal(err)
		}
	}

	j, err := json.MarshalIndent(configuration, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(j))
}
