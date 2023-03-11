/*
AnyStore is a thread-safe key/value store utilizing map[any]any in the
background with atomic.Value on read and mutex locks on write for performance.
Each map can optionally be persisted to disk as an encrypted GOB file. For
access from multiple instances sharing the same map, POSIX syscall.Flock is used
to exclusively lock the file during load and save. There is no support for
Windows or other non-POSIX systems without flock(2).
*/
package anystore

import (
	"crypto/rand"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
)

// A thread-safe key/value store using string as key and interface{} (any) as
// values. Must be initialized using NewAnyStore.
//
// Solution/inspiration from https://pkg.go.dev/sync/atomic#example-Value-ReadMostly
type AnyStore interface {
	EnablePersistence(file string) AnyStore

	DisablePersistence() AnyStore

	// HasKey tests if key exists in the store, returns true if it does, false if
	// not. Retrieval is atomic.
	HasKey(key any) bool

	// Load atomically retrieves the value of key. Returns the value as an
	// interface (any) and can therefore be casted into the correct type.
	Load(key any) any

	// Store adds or replaces a key/value pair in the store. Operation is locking
	// and more costly than Load or HasKey.
	Store(key any, value any)

	// Delete removes a key from the store. Operation uses sync.Mutex and is
	// locking.
	Delete(key any)

	// Len returns number of keys in the store.
	Len() int

	// Returns a slice with all keys in the store.
	Keys() []any

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
}

type anyStore struct {
	mutex       sync.Mutex
	kv          atomic.Value
	persist     atomic.Bool
	persistence atomic.Value
}

// Implements AnyStore and "overrides" Store, Delete and Run.
type unsafeAnyStore struct {
	*anyStore
}

// anyMap is a convenience-type.
type anyMap map[any]any

// NewAnyStore returns an initialized AnyStore.
func NewAnyStore() AnyStore {
	a := new(anyStore)
	a.kv.Store(make(anyMap))
	return a
}

func (a *anyStore) EnablePersistence(file string) AnyStore {
	a.mutex.Lock() // We want both boolean and string to be atomic operations.
	defer a.mutex.Unlock()
	a.persist.Store(true)
	a.persistence.Store(file)
	return a
}

func (a *anyStore) DisablePersistence() AnyStore {
	a.persist.Store(false)
	return a
}

func (a *anyStore) HasKey(key any) bool {
	kv := a.kv.Load().(anyMap)
	_, ok := kv[key]
	return ok
}

func (a *anyStore) Load(key any) any {
	kv := a.kv.Load().(anyMap)
	return kv[key]
}

func (a *anyStore) Store(key any, value any) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	kvO := a.kv.Load().(anyMap)
	kvN := make(anyMap)
	for k, v := range kvO {
		kvN[k] = v
	}
	kvN[key] = value
	a.kv.Store(kvN)

	if a.persist.Load() {
		// Persist...
	}
}

func (a *anyStore) Delete(key any) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	kvO := a.kv.Load().(anyMap)
	kvN := make(anyMap)
	for k, v := range kvO {
		kvN[k] = v
	}
	delete(kvN, key)
	a.kv.Store(kvN)
}

func (a *anyStore) Len() int {
	return len(a.kv.Load().(anyMap))
}

func (a *anyStore) Keys() []any {
	keys := make([]any, 0)
	kv, ok := a.kv.Load().(anyMap)
	if ok {
		for k, _ := range kv {
			keys = append(keys, k)
		}
	}
	return keys
}

func (a *anyStore) Run(atomicOperation func(a AnyStore) error) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	anyStoreOverride := &unsafeAnyStore{a}
	return atomicOperation(anyStoreOverride)
}

// unsafeAnyStore implements AnyStore, but in an unlocked state (where Store,
// Delete and Run have been modified not to lock) to be used in the Run
// function. All functions need to defined to implement the AnyStore interface.

func (u *unsafeAnyStore) EnablePersistence(file string) AnyStore {
	u.persist.Store(true)
	u.persistence.Store(file)
	return u
}

func (u *unsafeAnyStore) DisablePersistence() AnyStore {
	u.persist.Store(false)
	return u
}

func (u *unsafeAnyStore) HasKey(key any) bool {
	kv := u.kv.Load().(anyMap)
	_, ok := kv[key]
	return ok
}

func (u *unsafeAnyStore) Load(key any) any {
	kv := u.kv.Load().(anyMap)
	return kv[key]
}

func (u *unsafeAnyStore) Store(key any, value any) {
	kvO := u.kv.Load().(anyMap)
	kvN := make(anyMap)
	for k, v := range kvO {
		kvN[k] = v
	}
	kvN[key] = value
	u.kv.Store(kvN)
}

func (u *unsafeAnyStore) Delete(key any) {
	kvO := u.kv.Load().(anyMap)
	kvN := make(anyMap)
	for k, v := range kvO {
		kvN[k] = v
	}
	delete(kvN, key)
	u.kv.Store(kvN)
}

func (u *unsafeAnyStore) Len() int {
	return len(u.kv.Load().(anyMap))
}

func (u *unsafeAnyStore) Run(atomicOperation func(a AnyStore) error) error {
	return atomicOperation(u)
}

// Persistence

func rndstr(length int) string {
	buf := make([]byte, length)
	retries := 50
	for i := 0; i < retries; i++ {
		if _, err := rand.Read(buf); err != nil {
			fmt.Println("continuing")
			continue
		}
		break
	}
	return hex.EncodeToString(buf)
}

func save(file string, kv anyMap) error {
	// open file
	// syscall.Flock file LOCK_EX
	// create file.randomstuffbehindit
	// syscall.Flock created file
	// write to file.randomstuffbehindit
	// rename file.randomstuffbehindit to file
	// close file.randomstuf...
	// close file

	// 1. syscall.Open(file, syscall.O_CREAT | syscall.O_RDONLY, 0666)
	// 2.

	fd, err := syscall.Open(file, syscall.O_CREAT|syscall.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		return err
	}

	newFile := file + "." + rndstr(10)
	nfd, err := syscall.Open(newFile, syscall.O_CREAT|syscall.O_TRUNC|syscall.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer syscall.Close(nfd)
	if err := syscall.Flock(nfd, syscall.LOCK_EX); err != nil {
		return err
	}

	gob.GobDecoder


	return nil
}
