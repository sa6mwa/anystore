package main

import (
	"log"

	"github.com/sa6mwa/anystore"
)

type Thing struct {
	Name        *string
	Description string
	Number      int
	Components  []*Component
}

type Component struct {
	ID   int
	Name string
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC | log.Lshortfile)

	actualThing := &Thing{}

	defaultThing := &Thing{
		Name:        &[]string{"Hello World"}[0],
		Description: "There is not much to a Hello World thing.",
		Components: []*Component{
			{ID: 1, Name: "Component one"},
			{ID: 2, Name: "Component two"},
			{ID: 3, Name: "Component three"},
		},
	}

	file := "~/.anystore/examples-edit-stash.db"

	conf := &anystore.StashConfig{
		File:          file,
		GZip:          true,
		EncryptionKey: anystore.DefaultEncryptionKey,
		Key:           "configuration",
		Thing:         actualThing,
		DefaultThing:  defaultThing,
	}

	if err := anystore.Unstash(conf); err != nil {
		log.Fatal(err)
	}

	if err := anystore.EditThing(conf); err != nil {
		log.Fatal(err)
	}

	var gotThing Thing

	if err := anystore.Unstash(&anystore.StashConfig{
		File:          file,
		GZip:          true,
		EncryptionKey: anystore.DefaultEncryptionKey,
		Key:           "configuration",
		Thing:         &gotThing,
	}); err != nil {
		log.Fatal(err)
	}

	log.Printf("Saved configuration in %s", file)
}
