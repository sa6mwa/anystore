package anystore_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/sa6mwa/anystore"
)

func TestAnyStore_Run_persisted(t *testing.T) {
	f, err := os.CreateTemp("", "anystore-test-run-*")
	if err != nil {
		fmt.Println(err)
		return
	}
	tempfile := f.Name()
	f.Close()
	defer func() {
		os.Remove(tempfile)
		os.Remove(tempfile + ".lock")
	}()

	errTesting := errors.New("this error")
	a, err := anystore.NewAnyStore(&anystore.Options{
		EnablePersistence: true,
		PersistenceFile:   tempfile,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Store("hello", "world"); err != nil {
		t.Fatal(err)
	}
	if !a.HasKey("hello") {
		t.Fatal("expected key not found in store (outside of Run)")
	}

	err = a.Run(func(as anystore.AnyStore) error {
		if !as.HasKey("hello") {
			t.Error("expected key not found in store")
		}
		if err := as.Store(struct{}{}, "okilidokili"); err != nil {
			t.Fatal(err)
		}
		val, err := as.Load(struct{}{})
		if err != nil {
			t.Fatal(err)
		}
		v, ok := val.(string)
		if !ok {
			t.Fatalf("expected key \"struct{}{}\" with value of type string not found in store")
		}
		if v != "okilidokili" {
			t.Errorf("expected okilidokili, but got %q", v)
		}

		if l, err := as.Len(); err != nil {
			t.Fatal(err)
		} else if l != 2 {
			t.Errorf("expected Len() == %d, got %d", 2, l)
		}
		as.Store(struct{}{}, "completely")
		as.Store(struct{}{}, "different")
		as.Delete(struct{}{})
		if l, err := as.Len(); err != nil {
			t.Fatal(err)
		} else if l != 1 {
			t.Errorf("expected Len() == %d, got %d", 2, l)
		}
		if err := as.Store(struct{}{}, "okilidokili"); err != nil {
			t.Fatal(err)
		}
		return errTesting
	})
	if err != errTesting {
		t.Errorf("expected error %v, but got %v", errTesting, err)
	}
	o, err := a.Load(struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	expected := "okilidokili"
	if o != expected {
		t.Fatalf("expected key %q with value %q not found in store", "struct{}{}", expected)
	}
	nilVal, err := a.Load("keyNotPresent")
	if err != nil {
		t.Fatal(err)
	}
	if nilVal != nil {
		t.Errorf("expected nil, but got %T", nilVal)
	}
}

func TestAnyStore_Run(t *testing.T) {
	errTesting := errors.New("this error")
	a, err := anystore.NewAnyStore(&anystore.Options{
		EnablePersistence: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Store("hello", "world"); err != nil {
		t.Fatal(err)
	}
	if !a.HasKey("hello") {
		t.Fatal("expected key not found in store (outside of Run)")
	}

	err = a.Run(func(as anystore.AnyStore) error {
		if !as.HasKey("hello") {
			t.Error("expected key not found in store")
		}
		if err := as.Store(struct{}{}, "okilidokili"); err != nil {
			t.Fatal(err)
		}
		val, err := as.Load(struct{}{})
		if err != nil {
			t.Fatal(err)
		}
		v, ok := val.(string)
		if !ok {
			t.Fatalf("expected key \"struct{}{}\" with value of type string not found in store")
		}
		if v != "okilidokili" {
			t.Errorf("expected okilidokili, but got %q", v)
		}

		if l, err := as.Len(); err != nil {
			t.Fatal(err)
		} else if l != 2 {
			t.Errorf("expected Len() == %d, got %d", 2, l)
		}
		as.Store(struct{}{}, "completely")
		as.Store(struct{}{}, "different")
		as.Delete(struct{}{})
		if l, err := as.Len(); err != nil {
			t.Fatal(err)
		} else if l != 1 {
			t.Errorf("expected Len() == %d, got %d", 2, l)
		}
		if err := as.Store(struct{}{}, "okilidokili"); err != nil {
			t.Fatal(err)
		}
		return errTesting
	})
	if err != errTesting {
		t.Errorf("expected error %v, but got %v", errTesting, err)
	}
	o, err := a.Load(struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	expected := "okilidokili"
	if o != expected {
		t.Fatalf("expected key %q with value %q not found in store", "struct{}{}", expected)
	}
	nilVal, err := a.Load("keyNotPresent")
	if err != nil {
		t.Fatal(err)
	}
	if nilVal != nil {
		t.Errorf("expected nil, but got %T", nilVal)
	}
}

func TestAnyStore_GetEncryptionKeyBytes(t *testing.T) {
	expected, err := base64.RawStdEncoding.DecodeString(anystore.DefaultEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	a, err := anystore.NewAnyStore(&anystore.Options{
		EnablePersistence: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	obtained := a.GetEncryptionKeyBytes()
	if !bytes.Equal(expected, obtained) {
		t.Error("obtained bytes from AnyStore.GetEncryptionKeyBytes() and expected bytes do not match")
	}
}

func ExampleAnyStore_Store_encrypt() {
	f, err := os.CreateTemp("", "anystore-example-*")
	if err != nil {
		fmt.Println(err)
		return
	}
	tempfile := f.Name()
	f.Close()
	defer func() {
		os.Remove(tempfile)
		os.Remove(tempfile + ".lock")
	}()

	a, err := anystore.NewAnyStore(&anystore.Options{
		EnablePersistence: true,
		PersistenceFile:   tempfile,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	if err := a.Store("hello", "world"); err != nil {
		fmt.Println(err)
		return
	}

	if v, err := a.Load("hello"); err != nil {
		fmt.Println(err)
		return
	} else {
		val, ok := v.(string)
		if !ok {
			fmt.Println("val is not a string")
			return
		}
		fmt.Println(val)
	}

	if k, err := a.Keys(); err != nil {
		fmt.Println(err)
		return
	} else {
		for _, ky := range k {
			fmt.Println(ky)
		}
	}

	if l, err := a.Len(); err != nil {
		fmt.Println(err)
		return
	} else {
		fmt.Println(l)
	}

	// Output:
	// world
	// hello
	// 1
}

func BenchmarkStoreAndLoadPersistence(b *testing.B) {
	f, err := os.CreateTemp("", "anystore-benchmark-*")
	if err != nil {
		fmt.Println(err)
		return
	}
	tempfile := f.Name()
	f.Close()
	defer func() {
		os.Remove(tempfile)
		os.Remove(tempfile + ".lock")
	}()

	a, err := anystore.NewAnyStore(&anystore.Options{
		EnablePersistence: true,
		PersistenceFile:   tempfile,
	})
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		value := fmt.Sprintf("%s-%d", b.Name(), b.N)
		if err := a.Store(b.N, value); err != nil {
			b.Fatal(err)
		}
		if v, err := a.Load(b.N); err != nil {
			b.Fatal(err)
		} else {
			val, ok := v.(string)
			if !ok {
				b.Fatal("value is not a string")
			}
			if val != value {
				b.Fatal("value does not match expected string")
			}
		}
	}
}

func BenchmarkStoreAndLoadGZippedPersistence(b *testing.B) {
	f, err := os.CreateTemp("", "anystore-benchmark-*")
	if err != nil {
		fmt.Println(err)
		return
	}
	tempfile := f.Name()
	f.Close()
	defer func() {
		os.Remove(tempfile)
		os.Remove(tempfile + ".lock")
	}()

	a, err := anystore.NewAnyStore(&anystore.Options{
		EnablePersistence:   true,
		GZipPersistenceFile: true,
		PersistenceFile:     tempfile,
	})
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		value := fmt.Sprintf("%s-%d", b.Name(), b.N)
		if err := a.Store(b.N, value); err != nil {
			b.Fatal(err)
		}
		if v, err := a.Load(b.N); err != nil {
			b.Fatal(err)
		} else {
			val, ok := v.(string)
			if !ok {
				b.Fatal("value is not a string")
			}
			if val != value {
				b.Fatal("value does not match expected string")
			}
		}
	}
}

func BenchmarkStoreAndLoad(b *testing.B) {
	a, err := anystore.NewAnyStore(&anystore.Options{
		EnablePersistence: false,
	})
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		value := fmt.Sprintf("%s-%d", b.Name(), b.N)
		if err := a.Store(b.N, value); err != nil {
			b.Fatal(err)
		}
		if v, err := a.Load(b.N); err != nil {
			b.Fatal(err)
		} else {
			val, ok := v.(string)
			if !ok {
				b.Fatal("value is not a string")
			}
			if val != value {
				b.Fatal("value does not match expected string")
			}
		}
	}
}

func FuzzConcurrentPersistence(f *testing.F) {

	f.Add(1, false, "hello world")

	f.Fuzz(func(t *testing.T, count int, gzip bool, valueToStore string) {
		file, err := os.CreateTemp("", "anystore-concurrent-fuzz-*")
		if err != nil {
			fmt.Println(err)
			return
		}
		tempfile := file.Name()
		file.Close()
		defer func() {
			os.Remove(tempfile)
			os.Remove(tempfile + ".lock")
		}()

		ch1 := make(chan struct{})
		ch2 := make(chan struct{})

		go func() {
			defer close(ch1)
			one, err := anystore.NewAnyStore(&anystore.Options{
				EnablePersistence:   true,
				PersistenceFile:     tempfile,
				GZipPersistenceFile: gzip,
			})
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < count; i++ {
				key := fmt.Sprintf("one-%d", i)
				value := valueToStore
				if err := one.Store(key, value); err != nil {
					t.Fatal(err)
				}
				if v, err := one.Load(key); err != nil {
					t.Fatal(err)
				} else {
					val, ok := v.(string)
					if !ok {
						t.Fatalf("value %q is not a string (expected %q=%q)", val, key, value)
					}
					if val != value {
						t.Fatalf("value %q does not match expected string %q", val, value)
					}
				}
			}
		}()

		go func() {
			defer close(ch2)
			two, err := anystore.NewAnyStore(&anystore.Options{
				EnablePersistence:   true,
				PersistenceFile:     tempfile,
				GZipPersistenceFile: gzip,
			})
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < count; i++ {
				key := fmt.Sprintf("two-%d", i)
				value := valueToStore
				if err := two.Store(key, value); err != nil {
					t.Fatal(err)
				}
				if v, err := two.Load(key); err != nil {
					t.Fatal(err)
				} else {
					val, ok := v.(string)
					if !ok {
						t.Fatalf("value %q is not a string (expected %q=%q)", val, key, value)
					}
					if val != value {
						t.Fatalf("value %q does not match expected string %q", val, value)
					}
				}
			}
		}()

		<-ch1
		<-ch2
	})

}
