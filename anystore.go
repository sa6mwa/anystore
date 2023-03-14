/*
AnyStore is a thread-safe key/value store utilizing map[any]any in the
background with atomic.Value on read and mutex locks on write for performance.
The AnyStore map can optionally be persisted to disk as an AES-128/192/256
encrypted GOB file. For access from multiple instances sharing the same map,
POSIX syscall.Flock is used to exclusively lock a lockfile during save. There is
no support for Windows or other non-POSIX systems without flock(2).

Example:

	ephemeral, err := anystore.NewAnyStore(&anystore.Options{
		EnablePersistence: false,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Store this key in a safe place, share between instances
	encryptionKey := anystore.NewKey()

	persisted, err := anystore.NewAnyStore(&anystore.Options{
		EnablePersistence: true,
		PersistenceFile: "~/.persisted-data.db",
		EncryptionKey: encryptionKey,
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := ephemeral.Store("hello", "world"); err != nil {
		log.Fatal(err)
	}
	value, err := ephemeral.Load("hello")
	if err != nil {
		log.Fatal(err)
	}
	v, ok := value.(string)
	if !ok {
		log.Fatalf("value %q is not a string", value)
	}
	log.Println(v)

	// Persistence works just like a non-persisted store...

	if err := persisted.Store("hello", "world"); err != nil {
		log.Fatal(err)
	}
	value, err := persisted.Load("hello")
	if err != nil {
		log.Fatal(err)
	}
	v, ok := value.(string)
	if !ok {
		log.Fatalf("value %q is not a string", value)
	}
	log.Println(v)

	}

AnyStore also feature a configuration mode with convenience-functions Stash,
Unstash and EditThing. Whether you choose to hard-code an encryption key in the
application or provide one via environment variables, using Stash, Unstash and
EditThing is simple...

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
*/
package anystore

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
)

const DefaultEncryptionKey string = "cTAvflqncVmYD7bLM31fP3TVuwEoosMMwehpIwn1P84"

const DefaultPersistenceFile string = "~/.config/anystore/anystore.db"

var (
	ErrKeyLength      error = errors.New("key length must be 16, 24 or 32 (for AES-128, AES-192 or AES-256)")
	ErrWroteTooLittle error = errors.New("wrote too few bytes")
)

// A thread-safe key/value store using string as key and interface{} (any) as
// values. Must be initialized using NewAnyStore.
//
// Solution/inspiration from https://pkg.go.dev/sync/atomic#example-Value-ReadMostly
type AnyStore interface {
	SetPersistenceFile(file string) (AnyStore, error)

	EnablePersistence() AnyStore

	DisablePersistence() AnyStore

	SetEncryptionKey(key string) (AnyStore, error)

	// HasKey tests if key exists in the store, returns true if it does, false if
	// not. Retrieval is atomic.
	HasKey(key any) bool

	// Load atomically retrieves the value of key. Returns the value as an
	// interface (any) and can therefore be casted into the correct type.
	Load(key any) (any, error)

	// Store adds or replaces a key/value pair in the store. Operation is locking
	// and more costly than Load or HasKey.
	Store(key any, value any) error

	// Delete removes a key from the store. Operation uses sync.Mutex and is
	// locking.
	Delete(key any) error

	// Len returns number of keys in the store.
	Len() (int, error)

	// Returns a slice with all keys in the store.
	Keys() ([]any, error)

	// Run executes function atomicOperation exclusively by locking the store.
	// atomicOperation is intended to be an inline function running a set of
	// operations on the store in an exclusive scope. BEWARE! You have to use the
	// a AnyStore passed as argument to atomicOperation - it is not the same
	// struct and methods underneath as Store, Delete and Run are now non-blocking
	// ("unsafe") in "a". If you use the origin instance interface the Run
	// receiver function is attached to Store, Delete and Run will cause a
	// deadlock (these are overridden with non-locking versions in the interface
	// passed to atomicOperation). Technically, you could use the original HasKey,
	// Load and Len as they are non-locking and mere duplicates in the wrapped
	// instance, but that could cause confusion. The error returned by the passed
	// function is returned by Run.
	Run(atomicOperation func(a AnyStore) error) error

	load() error

	loadStoreAndSave(key any, value any, remove bool) error
}

type Options struct {
	// Store and load the AnyStore from file? Set to true
	EnablePersistence bool
	// Can start with tilde for HOME resolution, will do os.MkdirAll on directory
	// path. Omit to use DefaultPersistenceFile
	PersistenceFile string
	// 16, 24 or 32 byte base64-encoded string (omit to use the default key ==
	// insecure)
	EncryptionKey string
}

type anyStore struct {
	mutex    sync.Mutex
	kv       atomic.Value
	persist  atomic.Bool
	key      atomic.Value
	savefile atomic.Value
}

// Implements AnyStore and "overrides" Store, Delete and Run.
type unsafeAnyStore struct {
	*anyStore
}

// anyMap is a convenience-type.
type anyMap map[any]any

func init() {
	gob.Register([]interface{}(nil))
	gob.Register([]struct{}(nil))
	gob.Register(struct{}{})
}

// NewAnyStore returns an initialized AnyStore.
func NewAnyStore(o *Options) (AnyStore, error) {
	a := new(anyStore)
	if o == nil {
		o = &Options{}
	}
	if o.EnablePersistence {
		if o.PersistenceFile != "" {
			if _, err := a.SetPersistenceFile(o.PersistenceFile); err != nil {
				return a, err
			}
		} else {
			if _, err := a.SetPersistenceFile(DefaultPersistenceFile); err != nil {
				return a, err
			}
		}
		a.persist.Store(true)
	} else {
		a.persist.Store(false)
	}
	if o.EncryptionKey != "" {
		if _, err := a.SetEncryptionKey(o.EncryptionKey); err != nil {
			return a, err
		}
	} else {
		if _, err := a.SetEncryptionKey(DefaultEncryptionKey); err != nil {
			return a, err
		}
	}
	a.kv.Store(make(anyMap))
	return a, nil
}

func (a *anyStore) SetPersistenceFile(file string) (AnyStore, error) {
	// If persistence file starts with a tilde, resolve it to the user's home
	// directory.
	if strings.HasPrefix(file, "~/") {
		dirname, err := os.UserHomeDir()
		if err != nil {
			return a, err
		}
		file = filepath.Join(dirname, file[2:])
	}
	dir, _ := filepath.Split(file)
	if _, err := os.Stat(file); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(dir, 0777); err != nil {
				return a, err
			}
		} else {
			return a, err
		}
	}

	f, err := os.OpenFile(file, os.O_RDWR, 0666)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return a, err
		}
	} else {
		f.Close()
	}
	a.savefile.Store(file)
	return a, nil
}

func (a *anyStore) EnablePersistence() AnyStore {
	a.persist.Store(true)
	return a
}

func (a *anyStore) DisablePersistence() AnyStore {
	a.persist.Store(false)
	return a
}

func (a *anyStore) SetEncryptionKey(key string) (AnyStore, error) {
	binkey, err := base64.RawStdEncoding.DecodeString(key)
	if err != nil {
		return a, err
	}
	switch len(binkey) {
	case 16, 24, 32:
	default:
		return a, ErrKeyLength
	}
	a.key.Store(binkey)
	return a, nil
}

func (a *anyStore) HasKey(key any) bool {
	if a.persist.Load() {
		a.mutex.Lock()
		defer a.mutex.Unlock()
		a.load()
	}
	kv := a.kv.Load().(anyMap)
	_, ok := kv[key]
	return ok
}

func (a *anyStore) Load(key any) (any, error) {
	if a.persist.Load() {
		a.mutex.Lock()
		defer a.mutex.Unlock()
		// File is our only source of truth, load file before loading key
		if err := a.load(); err != nil {
			return nil, err
		}
	}
	kv := a.kv.Load().(anyMap)
	return kv[key], nil
}

func (a *anyStore) Store(key any, value any) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	if a.persist.Load() {
		return a.loadStoreAndSave(key, value, false)
	}
	kvO := a.kv.Load().(anyMap)
	kvN := make(anyMap)
	for k, v := range kvO {
		kvN[k] = v
	}
	kvN[key] = value
	a.kv.Store(kvN)
	return nil
}

func (a *anyStore) Delete(key any) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	if a.persist.Load() {
		return a.loadStoreAndSave(key, nil, true)
	}
	kvO := a.kv.Load().(anyMap)
	kvN := make(anyMap)
	for k, v := range kvO {
		kvN[k] = v
	}
	delete(kvN, key)
	a.kv.Store(kvN)
	return nil
}

func (a *anyStore) Len() (int, error) {
	if a.persist.Load() {
		a.mutex.Lock()
		defer a.mutex.Unlock()
		if err := a.load(); err != nil {
			return 0, err
		}
	}
	return len(a.kv.Load().(anyMap)), nil
}

func (a *anyStore) Keys() ([]any, error) {
	if a.persist.Load() {
		a.mutex.Lock()
		defer a.mutex.Unlock()
		if err := a.load(); err != nil {
			return nil, err
		}
	}
	keys := make([]any, 0)
	kv, ok := a.kv.Load().(anyMap)
	if ok {
		for k, _ := range kv {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (a *anyStore) Run(atomicOperation func(a AnyStore) error) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	anyStoreOverride := &unsafeAnyStore{a}
	return atomicOperation(anyStoreOverride)
}

func (a *anyStore) load() error {
	file, ok := a.savefile.Load().(string)
	if !ok {
		return errors.New("no persistence file set")
	}
	encryptionKey, ok := a.key.Load().([]byte)
	if !ok {
		return errors.New("encryption key not set")
	}
	// lockfile := file + ".lock"
	// lockfd, err := syscall.Open(lockfile, syscall.O_CREAT|syscall.O_RDWR, 0666)
	// if err != nil {
	// 	return err
	// }
	// defer syscall.Close(lockfd)
	// if err := syscall.Flock(lockfd, syscall.LOCK_EX); err != nil {
	// 	return err
	// }
	data := []byte{}
	f, err := os.OpenFile(file, os.O_RDONLY, 0666)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else {
		data, err = io.ReadAll(f)
		f.Close()
		if err != nil {
			return err
		}
	}
	kvN := make(anyMap)
	if len(data) > 0 {
		decrypted, err := Decrypt(encryptionKey, data)
		if err != nil {
			return err
		}
		if len(decrypted) > 0 {
			in := gob.NewDecoder(bytes.NewReader(decrypted))
			if err := in.Decode(&kvN); err != nil {
				return err
			}
		}
	}
	a.kv.Store(kvN)
	return nil
}

func (a *anyStore) loadStoreAndSave(key any, value any, remove bool) error {
	encryptionKey := a.key.Load().([]byte)
	file, ok := a.savefile.Load().(string)
	if !ok {
		return errors.New("persistence file not set")
	}
	lockfile := file + ".lock"
	unlink := true
	lockfd, err := syscall.Open(lockfile, syscall.O_CREAT|syscall.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer syscall.Close(lockfd)
	if err := syscall.Flock(lockfd, syscall.LOCK_EX); err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	// Make a new KV map
	kvN := make(anyMap)
	if len(data) > 0 {
		decrypted, err := Decrypt(encryptionKey, data)
		if err != nil {
			return err
		}
		if len(decrypted) > 0 {
			in := gob.NewDecoder(bytes.NewReader(decrypted))
			if err := in.Decode(&kvN); err != nil {
				return err
			}
		}
	}
	// Set our key/value on top of incoming KV pairs, or delete the key
	if remove {
		delete(kvN, key)
	} else {
		kvN[key] = value
	}
	// Store map
	a.kv.Store(kvN)
	// Store as GOB, encrypt it and save as temporary file along-side the original
	// and replace the main file via rename (as rename is atomic, it will not
	// corrupt the main file in the event of a crash).
	var gobOutput bytes.Buffer
	out := gob.NewEncoder(&gobOutput)
	if err := out.Encode(kvN); err != nil {
		return err
	}
	encryptedOutput, err := Encrypt(encryptionKey, gobOutput.Bytes())
	if err != nil {
		return err
	}
	newFilename := file + "." + rndstr(10)
	tmpf, err := os.OpenFile(newFilename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer func() {
		if unlink {
			os.Remove(newFilename)
		}
	}()
	if n, err := tmpf.Write(encryptedOutput); err != nil {
		tmpf.Close()
		return err
	} else if n != len(encryptedOutput) {
		tmpf.Close()
		return ErrWroteTooLittle
	}
	tmpf.Sync()
	tmpf.Close()
	if err := os.Rename(newFilename, file); err != nil {
		return err
	}
	unlink = false
	return nil
}

// unsafeAnyStore implements AnyStore, but in an unlocked state (where Store,
// Delete and Run have been modified not to lock) to be used in the Run
// function. All functions need to defined to implement the AnyStore interface.

func (u *unsafeAnyStore) SetPersistenceFile(file string) (AnyStore, error) {
	// If persistence file starts with a tilde, resolve it to the user's home
	// directory.
	if strings.HasPrefix(file, "~/") {
		dirname, err := os.UserHomeDir()
		if err != nil {
			return u, err
		}
		file = filepath.Join(dirname, file[2:])
	}
	dir, _ := filepath.Split(file)
	if _, err := os.Stat(file); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(dir, 0777); err != nil {
				return u, err
			}
		} else {
			return u, err
		}
	}
	f, err := os.OpenFile(file, os.O_RDWR, 0666)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return u, err
		}
	} else {
		f.Close()
	}
	u.savefile.Store(file)
	return u, nil
}

func (u *unsafeAnyStore) EnablePersistence() AnyStore {
	u.persist.Store(true)
	return u
}

func (u *unsafeAnyStore) DisablePersistence() AnyStore {
	u.persist.Store(false)
	return u
}

func (u *unsafeAnyStore) SetEncryptionKey(key string) (AnyStore, error) {
	binkey, err := base64.RawStdEncoding.DecodeString(key)
	if err != nil {
		return u, err
	}
	switch len(binkey) {
	case 16, 24, 32:
	default:
		return u, ErrKeyLength
	}
	u.key.Store(binkey)
	return u, nil
}

func (u *unsafeAnyStore) HasKey(key any) bool {
	if u.persist.Load() {
		u.load()
	}
	kv := u.kv.Load().(anyMap)
	_, ok := kv[key]
	return ok
}

func (u *unsafeAnyStore) Load(key any) (any, error) {
	if u.persist.Load() {
		// File is our only source of truth, load file before loading key
		if err := u.load(); err != nil {
			return nil, err
		}
	}
	kv := u.kv.Load().(anyMap)
	return kv[key], nil
}

func (u *unsafeAnyStore) Store(key any, value any) error {
	if u.persist.Load() {
		return u.loadStoreAndSave(key, value, false)
	}
	kvO := u.kv.Load().(anyMap)
	kvN := make(anyMap)
	for k, v := range kvO {
		kvN[k] = v
	}
	kvN[key] = value
	u.kv.Store(kvN)
	return nil
}

func (u *unsafeAnyStore) Delete(key any) error {
	if u.persist.Load() {
		return u.loadStoreAndSave(key, nil, true)
	}
	kvO := u.kv.Load().(anyMap)
	kvN := make(anyMap)
	for k, v := range kvO {
		kvN[k] = v
	}
	delete(kvN, key)
	u.kv.Store(kvN)
	return nil
}

func (u *unsafeAnyStore) Len() (int, error) {
	if u.persist.Load() {
		if err := u.load(); err != nil {
			return 0, err
		}
	}
	return len(u.kv.Load().(anyMap)), nil
}

func (u *unsafeAnyStore) Keys() ([]any, error) {
	if u.persist.Load() {
		if err := u.load(); err != nil {
			return nil, err
		}
	}
	keys := make([]any, 0)
	kv, ok := u.kv.Load().(anyMap)
	if ok {
		for k, _ := range kv {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (u *unsafeAnyStore) Run(atomicOperation func(a AnyStore) error) error {
	return atomicOperation(u)
}

func (u *unsafeAnyStore) load() error {
	file, ok := u.savefile.Load().(string)
	if !ok {
		return errors.New("no persistence file set")
	}
	encryptionKey, ok := u.key.Load().([]byte)
	if !ok {
		return errors.New("encryption key not set")
	}
	// lockfile := file + ".lock"
	// lockfd, err := syscall.Open(lockfile, syscall.O_CREAT|syscall.O_RDWR, 0666)
	// if err != nil {
	// 	return err
	// }
	// defer syscall.Close(lockfd)
	// if err := syscall.Flock(lockfd, syscall.LOCK_EX); err != nil {
	// 	return err
	// }
	data := []byte{}
	f, err := os.OpenFile(file, os.O_RDONLY, 0666)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else {
		data, err = io.ReadAll(f)
		f.Close()
		if err != nil {
			return err
		}
	}
	kvN := make(anyMap)
	if len(data) > 0 {
		decrypted, err := Decrypt(encryptionKey, data)
		if err != nil {
			return err
		}
		if len(decrypted) > 0 {
			in := gob.NewDecoder(bytes.NewReader(decrypted))
			if err := in.Decode(&kvN); err != nil {
				return err
			}
		}
	}
	u.kv.Store(kvN)
	return nil
}

func (u *unsafeAnyStore) loadStoreAndSave(key any, value any, remove bool) error {
	encryptionKey := u.key.Load().([]byte)
	file, ok := u.savefile.Load().(string)
	if !ok {
		return errors.New("persistence file not set")
	}
	lockfile := file + ".lock"
	unlink := true
	lockfd, err := syscall.Open(lockfile, syscall.O_CREAT|syscall.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer syscall.Close(lockfd)
	if err := syscall.Flock(lockfd, syscall.LOCK_EX); err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	// Make a new KV map
	kvN := make(anyMap)
	if len(data) > 0 {
		decrypted, err := Decrypt(encryptionKey, data)
		if err != nil {
			return err
		}
		if len(decrypted) > 0 {
			in := gob.NewDecoder(bytes.NewReader(decrypted))
			if err := in.Decode(&kvN); err != nil {
				return err
			}
		}
	}
	// Set our key/value on top of incoming KV pairs, or delete the key
	if remove {
		delete(kvN, key)
	} else {
		kvN[key] = value
	}
	// Store map
	u.kv.Store(kvN)
	// Store as GOB, encrypt it and save as temporary file along-side the original
	// and replace the main file via rename (as rename is atomic, it will not
	// corrupt the main file in the event of a crash).
	var gobOutput bytes.Buffer
	out := gob.NewEncoder(&gobOutput)
	if err := out.Encode(kvN); err != nil {
		return err
	}
	encryptedOutput, err := Encrypt(encryptionKey, gobOutput.Bytes())
	if err != nil {
		return err
	}
	newFilename := file + "." + rndstr(10)
	tmpf, err := os.OpenFile(newFilename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer func() {
		if unlink {
			os.Remove(newFilename)
		}
	}()
	if n, err := tmpf.Write(encryptedOutput); err != nil {
		tmpf.Close()
		return err
	} else if n != len(encryptedOutput) {
		tmpf.Close()
		return ErrWroteTooLittle
	}
	tmpf.Sync()
	tmpf.Close()
	if err := os.Rename(newFilename, file); err != nil {
		return err
	}
	unlink = false
	return nil
}

// Functions related to persistence...

func rndstr(length int) string {
	buf := make([]byte, length)
	retries := 50
	for i := 0; i < retries; i++ {
		if _, err := rand.Read(buf); err != nil {
			continue
		}
		break
	}
	return hex.EncodeToString(buf)
}

// NewKey generates a 32 byte base64 encoded random string for use as an AES-256
// key. Get a new key from the command line:
//
//	go run github.com/sa6mwa/anystore/cmd/newkey
func NewKey() string {
	randomBytes := make([]byte, 32)
	retries := 50
	for i := 0; i < retries; i++ {
		if _, err := rand.Read(randomBytes); err != nil {
			continue
		}
		break
	}
	return base64.RawStdEncoding.EncodeToString(randomBytes)
}

func Encrypt(key []byte, data []byte) ([]byte, error) {
	// Maybe implement later, but comes from an external package...
	//dk := pbkdf2.Key(key, []byte(salt), 4096, 32, sha256.New)
	switch len(key) {
	case 16, 24, 32:
	default:
		return nil, ErrKeyLength
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	ciphered := make([]byte, aes.BlockSize+len(data))
	salt := ciphered[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	stream := cipher.NewCFBEncrypter(block, salt)
	stream.XORKeyStream(ciphered[aes.BlockSize:], data)
	return ciphered, nil
}

func Decrypt(key []byte, data []byte) ([]byte, error) {
	switch len(key) {
	case 16, 24, 32:
	default:
		return nil, ErrKeyLength
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(data) < aes.BlockSize {
		return nil, fmt.Errorf("data shorter than AES block size (%d)", aes.BlockSize)
	}
	salt := data[:aes.BlockSize]
	deciphered := make([]byte, len(data[aes.BlockSize:]))
	stream := cipher.NewCFBDecrypter(block, salt)
	stream.XORKeyStream(deciphered, data[aes.BlockSize:])
	return deciphered, nil
}
