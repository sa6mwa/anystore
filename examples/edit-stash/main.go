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
	thingToEdit := &Thing{
		Name:        &[]string{"Hello World"}[0],
		Number:      32,
		Description: "There is not much to a Hello World thing.",
		Components: []*Component{
			{ID: 1, Name: "Component one"},
			{ID: 2, Name: "Component two"},
			{ID: 3, Name: "Component three"},
		},
	}

	defaultThing := &Thing{
		Name:        &[]string{"default"}[0],
		Description: "the default thing",
		Components: []*Component{
			{ID: 1, Name: "hello"},
		},
	}

	file := "~/.testing-edit-stash.db"

	if err := anystore.EditThing(&anystore.StashConfig{
		File:          file,
		EncryptionKey: anystore.DefaultEncryptionKey,
		Key:           "configuration",
		Thing:         thingToEdit,
	}, defaultThing); err != nil {
		log.Fatal(err)
	}

	var gotThing Thing

	if err := anystore.Unstash(&anystore.StashConfig{
		File:          file,
		EncryptionKey: anystore.DefaultEncryptionKey,
		Key:           "configuration",
		Thing:         &gotThing,
	}, nil); err != nil {
		log.Fatal(err)
	}

	log.Printf("Saved configuration in %s", file)
}
