package anystore

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"reflect"
)

var (
	ErrNilThing      error = errors.New("nil pointer to thing to stash or unstash")
	ErrEmptyKey      error = errors.New("stash key can not be empty (zero-length key)")
	ErrNotAPointer   error = errors.New("stash configuration does not point to thing, need a pointer")
	ErrThingNotFound error = errors.New("thing not found in stash")
	ErrTypeMismatch  error = errors.New("type-mismatch between thing and default thing")
)

type StashConfig struct {
	File          string // AnyStore DB file
	EncryptionKey string // 16, 24 or 32 byte long base64-encoded string
	Key           string // Key name where to store Thing
	Thing         any    // Usually a struct with data, properties, configuration, etc
	Editor        string // Editor to use to edit Thing as JSON
}

func Unstash(conf *StashConfig, defaultThing any) error {
	if conf.Thing == nil {
		return ErrNilThing
	}
	if conf.Key == "" {
		return ErrEmptyKey
	}

	a, err := NewAnyStore(&Options{
		EnablePersistence: true,
		PersistenceFile:   conf.File,
		EncryptionKey:     conf.EncryptionKey,
	})
	if err != nil {
		return err
	}
	gobbedThing, err := a.Load(conf.Key)
	if err != nil {
		return err
	}
	thing, ok := gobbedThing.([]byte)
	if !ok {
		if defaultThing != nil {
			if reflect.TypeOf(conf.Thing) != reflect.TypeOf(defaultThing) {
				return fmt.Errorf("%w: %s != %s", ErrTypeMismatch, reflect.TypeOf(defaultThing), reflect.TypeOf(conf.Thing))
			}
			if reflect.TypeOf(conf.Thing).Kind() != reflect.Pointer || reflect.TypeOf(defaultThing).Kind() != reflect.Pointer {
				return ErrNotAPointer
			}
			reflect.Indirect(reflect.ValueOf(conf.Thing)).Set(reflect.Indirect(reflect.ValueOf(defaultThing)))
			return nil
		}
		return ErrThingNotFound
	}
	g := gob.NewDecoder(bytes.NewReader(thing))
	// Decode into wherever StashConfig.Thing is pointing to.
	if err := g.Decode(conf.Thing); err != nil {
		return err
	}
	return nil
}

// "stash, verb. to put (something of future use or value) in a safe or secret
// place"
func Stash(conf *StashConfig) error {
	value := reflect.ValueOf(conf.Thing)
	if value.Type().Kind() != reflect.Pointer {
		return ErrNotAPointer
	}
	if value.IsNil() {
		return ErrNilThing
	}
	a, err := NewAnyStore(&Options{
		EnablePersistence: true,
		PersistenceFile:   conf.File,
		EncryptionKey:     conf.EncryptionKey,
	})
	if err != nil {
		return err
	}
	// Use gob to store the struct (or other value) instead of re-inventing
	// dereference of all pointers. It is also unlikely that the interface stored
	// is registered with gob in the downstream anystore package.
	var thing bytes.Buffer
	g := gob.NewEncoder(&thing)
	if err := g.Encode(conf.Thing); err != nil {
		return fmt.Errorf("gob.Encode of StashConfig.Thing: %w", err)
	}
	return a.Store(conf.Key, thing.Bytes())
}
