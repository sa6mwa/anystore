package anystore_test

import (
	"errors"
	"os"
	"reflect"
	"testing"

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

func strptr(s string) *string {
	return &s
}

func doStash(file string, encryptionKey string) error {
	expectedThing := &Thing{
		Name:        strptr("Hello World"),
		Number:      32,
		Description: "There is not much to a Hello World thing.",
		Components: []*Component{
			{ID: 1, Name: "Component one"},
			{ID: 2, Name: "Component two"},
			{ID: 3, Name: "Component three"},
		},
	}
	if err := anystore.Stash(&anystore.StashConfig{
		File:          file,
		EncryptionKey: encryptionKey,
		Key:           "configuration",
		Thing:         expectedThing,
	}); err != nil {
		return err
	}
	return nil
}

func doUnstash(file string, encryptionKey string) (Thing, error) {
	var gotThing Thing
	if err := anystore.Unstash(&anystore.StashConfig{
		File:          file,
		EncryptionKey: encryptionKey,
		Key:           "configuration",
		Thing:         &gotThing,
	}, nil); err != nil {
		return Thing{}, err
	}
	return gotThing, nil
}

func doUnstashDefault(file string, encryptionKey string) (Thing, error) {
	defaultThing := &Thing{
		Name:        strptr("Hello World"),
		Number:      32,
		Description: "There is not much to a Hello World thing.",
		Components: []*Component{
			{ID: 1, Name: "Component one"},
			{ID: 2, Name: "Component two"},
			{ID: 3, Name: "Component three"},
		},
	}
	var gotThing Thing
	if err := anystore.Unstash(&anystore.StashConfig{
		File:          file,
		EncryptionKey: encryptionKey,
		Key:           "key_not_in_stash",
		Thing:         &gotThing,
	}, defaultThing); err != nil {
		return Thing{}, err
	}
	return gotThing, nil
}

func TestUnstash_stashAndUnstash(t *testing.T) {
	secret := anystore.NewKey()

	fl, err := os.CreateTemp("", "anystore-stash-test-*")
	if err != nil {
		t.Fatal(err)
	}
	tempfile := fl.Name()
	fl.Close()
	defer func() {
		os.Remove(tempfile)
		os.Remove(tempfile + ".lock")
	}()

	expectedThing := &Thing{
		Name:        strptr("Hello World"),
		Number:      32,
		Description: "There is not much to a Hello World thing.",
		Components: []*Component{
			{ID: 1, Name: "Component one"},
			{ID: 2, Name: "Component two"},
			{ID: 3, Name: "Component three"},
		},
	}

	var gotThing Thing

	if err := anystore.Stash(&anystore.StashConfig{
		File:          tempfile,
		EncryptionKey: secret,
		Key:           "configuration",
		Thing:         expectedThing,
	}); err != nil {
		t.Fatal(err)
	}

	if err := anystore.Unstash(&anystore.StashConfig{
		File:          tempfile,
		EncryptionKey: secret,
		Key:           "configuration",
		Thing:         &gotThing,
	}, nil); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(&gotThing, expectedThing) {
		t.Errorf("got %s and expected %s does not match", reflect.TypeOf(gotThing), reflect.TypeOf(expectedThing))
	}

	defaultThing := &Thing{
		Name:        strptr("default"),
		Description: "the default thing",
		Components: []*Component{
			{ID: 1, Name: "hello"},
		},
	}

	if err := anystore.Unstash(&anystore.StashConfig{
		File:          tempfile,
		EncryptionKey: secret,
		Key:           "key_that_does_not_exist",
		Thing:         &gotThing,
	}, defaultThing); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(&gotThing, defaultThing) {
		t.Errorf("got %s and expected %s does not match", reflect.TypeOf(gotThing), reflect.TypeOf(defaultThing))
	}
}

func TestStash(t *testing.T) {
	secret := anystore.NewKey()
	fl, err := os.CreateTemp("", "anystore-stash-test-*")
	if err != nil {
		t.Fatal(err)
	}
	tempfile := fl.Name()
	fl.Close()
	defer func() {
		os.Remove(tempfile)
		os.Remove(tempfile + ".lock")
	}()
	expectedThing := &Thing{
		Name:        strptr("Hello World"),
		Number:      32,
		Description: "There is not much to a Hello World thing.",
		Components: []*Component{
			{ID: 1, Name: "Component one"},
			{ID: 2, Name: "Component two"},
			{ID: 3, Name: "Component three"},
		},
	}
	if err := doStash(tempfile, secret); err != nil {
		t.Fatal(err)
	}
	gotThing, err := doUnstash(tempfile, secret)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(&gotThing, expectedThing) {
		t.Errorf("got %s and expected %s does not match", reflect.TypeOf(gotThing), reflect.TypeOf(expectedThing))
	}
}

func TestUnstash(t *testing.T) {
	secret := anystore.NewKey()
	fl, err := os.CreateTemp("", "anystore-stash-test-*")
	if err != nil {
		t.Fatal(err)
	}
	tempfile := fl.Name()
	fl.Close()
	defer func() {
		os.Remove(tempfile)
		os.Remove(tempfile + ".lock")
	}()
	expectedThing := &Thing{
		Name:        strptr("Hello World"),
		Number:      32,
		Description: "There is not much to a Hello World thing.",
		Components: []*Component{
			{ID: 1, Name: "Component one"},
			{ID: 2, Name: "Component two"},
			{ID: 3, Name: "Component three"},
		},
	}
	if err := doStash(tempfile, secret); err != nil {
		t.Fatal(err)
	}
	gotThing, err := doUnstash(tempfile, secret)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(&gotThing, expectedThing) {
		t.Errorf("got %s and expected %s does not match", reflect.TypeOf(gotThing), reflect.TypeOf(expectedThing))
	}
	gotThing, err = doUnstashDefault(tempfile, secret)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(&gotThing, expectedThing) {
		t.Errorf("got %s and expected %s does not match", reflect.TypeOf(gotThing), reflect.TypeOf(expectedThing))
	}

	var gotThing2 Thing
	if err := anystore.Unstash(&anystore.StashConfig{
		File:          tempfile,
		EncryptionKey: secret,
		Key:           "key_not_in_stash",
		Thing:         &gotThing2,
	}, nil); err != nil {
		if !errors.Is(err, anystore.ErrThingNotFound) {
			t.Error(err)
		}
	} else {
		t.Error("expected anystore.ErrThingNotFound")
	}
}
