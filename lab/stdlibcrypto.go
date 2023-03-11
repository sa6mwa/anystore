package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
)

const DefaultKey string = "cTAvflqncVmYD7bLM31fP3TVuwEoosMMwehpIwn1P84"

func main() {
	fmt.Println(newKey())

	str := "hello world, this is a very long string, or a long thing of text. we are testing if it will encrypt and decrypt this long string. If not, there is something wrong, and we will need to refactor the thing that does the crypto thing."

	enc, err := encrypt(DefaultKey, []byte(str))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(hex.EncodeToString(enc))

	fmt.Println("length is: ", len(enc))

	dec, err := decrypt(DefaultKey, enc)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(dec))
}

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

func newKey() string {
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

func encrypt(key string, data []byte) ([]byte, error) {
	// Maybe implement later, but comes from an external package...
	//dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)

	binkey, err := base64.RawStdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("key must be a base64 encoded string: %w", err)
	}
	switch len(binkey) {
	case 16, 24, 32:
	default:
		return nil, errors.New("key length must be 16, 24 or 32 (for AES-128, AES-192 or AES-256)")
	}
	block, err := aes.NewCipher(binkey)
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

func decrypt(key string, data []byte) ([]byte, error) {
	binkey, err := base64.RawStdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("key must be a base64 encoded string: %w", err)
	}
	switch len(binkey) {
	case 16, 24, 32:
	default:
		return nil, errors.New("key length must be 16, 24 or 32 (for AES-128, AES-192 or AES-256)")
	}
	block, err := aes.NewCipher(binkey)
	if err != nil {
		return nil, err
	}
	if len(data) < aes.BlockSize {
		return nil, fmt.Errorf("data shorter than AES block size")
	}
	salt := data[:aes.BlockSize]
	deciphered := data[aes.BlockSize:]
	stream := cipher.NewCFBDecrypter(block, salt)
	stream.XORKeyStream(deciphered, deciphered)
	return deciphered, nil
}
