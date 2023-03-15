package anystore_test

import (
	"errors"
	"fmt"
	"io"
	"log"
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

func doStash(file string, writer io.WriteCloser, encryptionKey string) error {
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

	stashconf := anystore.StashConfig{
		File:          file,
		EncryptionKey: encryptionKey,
		Key:           "configuration",
		Thing:         expectedThing,
		Writer:        writer,
	}

	if err := anystore.Stash(&stashconf); err != nil {
		return err
	}
	return nil
}

func doUnstash(file string, reader io.Reader, encryptionKey string) (Thing, error) {
	var gotThing Thing

	if err := anystore.Unstash(&anystore.StashConfig{
		File:          file,
		EncryptionKey: encryptionKey,
		Key:           "configuration",
		Thing:         &gotThing,
		Reader:        reader,
	}, nil); err != nil {
		return Thing{}, err
	}
	return gotThing, nil
}

func doUnstashDefault(file string, reader io.Reader, encryptionKey string) (Thing, error) {
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
		Reader:        reader,
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
	if err := doStash(tempfile, nil, secret); err != nil {
		t.Fatal(err)
	}
	gotThing, err := doUnstash(tempfile, nil, secret)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(&gotThing, expectedThing) {
		t.Errorf("got %s and expected %s does not match", reflect.TypeOf(gotThing), reflect.TypeOf(expectedThing))
	}
}

func TestStash_doublestash(t *testing.T) {
	secret := anystore.NewKey()
	fl, err := os.CreateTemp("", "anystore-stash-double-test-*")
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

	reader, writer := io.Pipe()
	defer reader.Close() // writer will be closed by Stash
	errch := make(chan error)
	go func() {
		defer close(errch)
		gt, err := doUnstash("", reader, secret)
		if err != nil {
			errch <- err
			return
		}
		if !reflect.DeepEqual(&gt, expectedThing) {
			errch <- fmt.Errorf("got %s and expected %s does not match", reflect.TypeOf(gt), reflect.TypeOf(expectedThing))
			return
		}
		errch <- nil
	}()
	if err := doStash(tempfile, writer, secret); err != nil {
		t.Fatal(err)
	}
	err = <-errch
	if err != nil {
		t.Fatal(err)
	}

	gotThing, err := doUnstash(tempfile, nil, secret)
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
	if err := doStash(tempfile, nil, secret); err != nil {
		t.Fatal(err)
	}
	gotThing, err := doUnstash(tempfile, nil, secret)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(&gotThing, expectedThing) {
		t.Errorf("got %s and expected %s does not match", reflect.TypeOf(gotThing), reflect.TypeOf(expectedThing))
	}
	gotThing, err = doUnstashDefault(tempfile, nil, secret)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(&gotThing, expectedThing) {
		t.Errorf("got %s and expected %s does not match", reflect.TypeOf(gotThing), reflect.TypeOf(expectedThing))
	}

	// Test io.Reader, should be the same as the expectedThing
	if err := doStash(tempfile, nil, secret); err != nil {
		t.Fatal(err)
	}
	tf, err := os.Open(tempfile)
	if err != nil {
		t.Fatal(err)
	}
	gotThing, err = doUnstash("", tf, secret)
	tf.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(&gotThing, expectedThing) {
		t.Errorf("got %s and expected %s does not match", reflect.TypeOf(gotThing), reflect.TypeOf(expectedThing))
	}

	// Test stashing to an io.Writer and unstash from an io.Reader using io.Pipe.
	reader, writer := io.Pipe()
	defer reader.Close() // writer will be closed by Stash
	errch := make(chan error)
	go func() {
		defer close(errch)
		gt, err := doUnstash("", reader, secret)
		if err != nil {
			errch <- err
			return
		}
		if !reflect.DeepEqual(&gt, expectedThing) {
			errch <- fmt.Errorf("got %s and expected %s does not match", reflect.TypeOf(gt), reflect.TypeOf(expectedThing))
			return
		}
		errch <- nil
	}()
	if err := doStash("", writer, secret); err != nil {
		t.Fatal(err)
	}
	err = <-errch
	if err != nil {
		t.Fatal(err)
	}

	// Test negative case (key not found)
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

func ExampleStash_reader_writer() {
	greeting := "Hello world"
	var receivedGreeting string

	reader, writer := io.Pipe()
	defer reader.Close() // Stash closes the writer, it's an io.ReadCloser

	errch := make(chan error)

	go func() {
		defer close(errch)
		if err := anystore.Unstash(&anystore.StashConfig{
			Reader: reader,
			Key:    "secret",
			Thing:  &receivedGreeting,
		}, nil); err != nil {
			errch <- err
		}
		errch <- nil
	}()

	if err := anystore.Stash(&anystore.StashConfig{
		Writer: writer,
		Key:    "secret",
		Thing:  &greeting,
	}); err != nil {
		log.Fatal(err)
	}

	err := <-errch
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(receivedGreeting)

	// Output:
	// Hello world
}
