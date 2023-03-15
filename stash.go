package anystore

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"reflect"
)

var (
	ErrNilThing      error = errors.New("nil pointer to thing to stash or unstash")
	ErrEmptyKey      error = errors.New("stash key can not be empty (zero-length key)")
	ErrNotAPointer   error = errors.New("stash configuration does not point to thing, need a pointer")
	ErrThingNotFound error = errors.New("thing not found in stash")
	ErrTypeMismatch  error = errors.New("type-mismatch between thing and default thing")
	ErrMissingReader error = errors.New("missing filename (or io.Reader) to load persisted data from")
	ErrMissingWriter error = errors.New("missing filename (or io.Writer) to persist data to")
)

// StashConfig instructs how functions anystore.Stash and anystore.Unstash
// should save/load a "stash". If Reader is not nil and File is not an empty
// string, Reader will be preferred over File when executing Unstash. If Writer
// is not nil and File is not an empty string when executing Stash, the file
// will be written first, then written to through the io.Writer (both will be
// written to). Writer.Close() is deferred early, Stash always closes the writer
// on success and failure. If File is an empty string (== "") and Writer is not
// nil, Stash will only write to the io.Writer.
type StashConfig struct {
	File          string         // AnyStore DB file, if empty, use Reader/Writer
	Reader        io.Reader      // If nil, use File for Unstash, if not, prefer Reader over File
	Writer        io.WriteCloser // If nil, use File for Stash, if not, write to both Writer and File (if File is not an empty string)
	EncryptionKey string         // 16, 24 or 32 byte long base64-encoded string
	Key           string         // Key name where to store Thing
	Thing         any            // Usually a struct with data, properties, configuration, etc
	Editor        string         // Editor to use to edit Thing as JSON
}

// "stash, verb. to put (something of future use or value) in a safe or secret
// place"
//
// Unstash loads a "Thing" from a place specified in a StashConfig, usually an
// AnyStore DB file, but the Stash and Unstash functions also support io.Reader
// and io.Writer (io.WriteCloser). Reader/writer is essentially an in-memory
// version of the physical DB file, Unstash does io.ReadAll into memory in order
// to decrypt and de-GOB the data. A previous file-Stash command can be
// Unstashed via the io.Reader. Unstash prefers io.Reader when both
// StashConfig.File and StashConfig.Reader are defined.
//
// StashConfig instructs how functions anystore.Stash and anystore.Unstash
// should save/load a "stash". If Reader is not nil and File is not an empty
// string, Reader will be preferred over File when executing Unstash. If Writer
// is not nil and File is not an empty string when executing Stash, the file
// will be written first, then written to through the io.Writer (both will be
// written to). Writer.Close() is deferred early, Stash always closes the writer
// on success and failure. If File is an empty string (== "") and Writer is not
// nil, Stash will only write to the io.Writer.
func Unstash(conf *StashConfig, defaultThing any) error {
	if conf.Thing == nil {
		return ErrNilThing
	}
	if conf.Key == "" {
		return ErrEmptyKey
	}
	if conf.File == "" && conf.Reader == nil {
		return ErrMissingReader
	}
	options := Options{
		EnablePersistence: true,
		PersistenceFile:   conf.File,
		EncryptionKey:     conf.EncryptionKey,
	}
	// If we have an io.Reader, prefer it above File.
	if conf.Reader != nil {
		options.EnablePersistence = false
	}
	a, err := NewAnyStore(&options)
	if err != nil {
		return err
	}
	var gobbedThing any
	if conf.Reader != nil {
		// Read encrypted anyMap
		kv := make(anyMap)
		data, err := io.ReadAll(conf.Reader)
		if err != nil {
			return err
		}
		decrypted, err := Decrypt(a.GetEncryptionKeyBytes(), data)
		if err != nil {
			return err
		}
		in := gob.NewDecoder(bytes.NewReader(decrypted))
		if err := in.Decode(&kv); err != nil {
			return err
		}
		var ok bool
		gobbedThing, ok = kv[conf.Key]
		if !ok {
			return ErrThingNotFound
		}
	} else {
		// Load key from PersistenceFile instead.
		var err error
		gobbedThing, err = a.Load(conf.Key)
		if err != nil {
			return err
		}
	}
	// GOB encoded thing came from either file or io.Reader.
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
//
// Stash stores a "Thing" according to a StashConfig, usually an AnyStore DB
// file, but Stash and Unstash can also be used with an io.Writer
// (io.WriteCloser) and an io.Reader for arbitrary stashing/unstashing. Stash
// always closes the writer on exit (why it's an io.WriteCloser). The
// reader/writers are essentially in-memory versions of the physical DB file,
// Unstash does io.ReadAll into memory in order to decrypt and de-GOB it.
//
// StashConfig instructs how functions anystore.Stash and anystore.Unstash
// should save/load a "stash". If Reader is not nil and File is not an empty
// string, Reader will be preferred over File when executing Unstash. If Writer
// is not nil and File is not an empty string when executing Stash, the file
// will be written first, then written to through the io.Writer (both will be
// written to). Writer.Close() is deferred early, Stash always closes the writer
// on success and failure. If File is an empty string (== "") and Writer is not
// nil, Stash will only write to the io.Writer.
func Stash(conf *StashConfig) error {
	if conf.Writer != nil {
		defer conf.Writer.Close()
	}
	value := reflect.ValueOf(conf.Thing)
	if value.Type().Kind() != reflect.Pointer {
		return ErrNotAPointer
	}
	if value.IsNil() {
		return ErrNilThing
	}
	if conf.Key == "" {
		return ErrEmptyKey
	}
	if conf.File == "" && conf.Writer == nil {
		return ErrMissingWriter
	}

	options := Options{
		PersistenceFile: conf.File,
		EncryptionKey:   conf.EncryptionKey,
	}
	if conf.File == "" {
		options.EnablePersistence = false
	} else {
		options.EnablePersistence = true
	}

	a, err := NewAnyStore(&options)
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
	// Persist to file if filename was not an empty string.
	if conf.File != "" {
		if err := a.Store(conf.Key, thing.Bytes()); err != nil {
			return err
		}
	}
	// If conf.Writer was given, also write to the io.Writer, but this has to be
	// emulated (AnyStore does not implement io.Writer or io.Reader).
	if conf.Writer != nil {
		kv := make(anyMap)
		kv[conf.Key] = thing.Bytes()
		var gobOutput bytes.Buffer
		out := gob.NewEncoder(&gobOutput)
		if err := out.Encode(kv); err != nil {
			return err
		}
		encrypted, err := Encrypt(a.GetEncryptionKeyBytes(), gobOutput.Bytes())
		if err != nil {
			return err
		}
		if n, err := conf.Writer.Write(encrypted); err != nil {
			return err
		} else if n != len(encrypted) {
			return ErrWroteTooLittle
		}
	}
	return nil
}
