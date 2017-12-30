package main

import (
	"crypto/sha256"
	"encoding/binary"
	"crypto/rsa"
	"crypto/rand"
	"encoding/base32"
	"strings"
	"os"
	"io/ioutil"
	"fmt"
	"encoding/gob"
	"bytes"
	"reflect"
)

const DATA_DIR = "_data"

// GenerateKeyPair generates a 2048-bit RSA public/private key pair
func GenerateKeyPair(vanityNameBin []byte) *rsa.PrivateKey {
	fmt.Println("Generating a 2048-bit RSA keypair for the first time.")


	if len(vanityNameBin) > 0 {
		// Generate a key whose derived display name starts with the requested vanity name
		fmt.Println("A vanity name has been requested. Bruteforcing it...")
	}

	var key *rsa.PrivateKey
	var err error
	for {
		key, err = rsa.GenerateKey(rand.Reader, 2048)
		FailOnError(err)

		// Found a valid public key
		if reflect.DeepEqual(ComputePublicKeyFingerprint(&key.PublicKey)[:len(vanityNameBin)], vanityNameBin) {
			break
		}
	}

	// Save the key on file
	dir := DATA_DIR + "/" + Context.ThisNodeAlias
	os.MkdirAll(dir, os.ModePerm)

	buf := bytes.Buffer{}
	encoder := gob.NewEncoder(&buf)
	err = encoder.Encode(key)
	FailOnError(err)

	err = ioutil.WriteFile(dir + "/key.bin", buf.Bytes(), 0644)
	FailOnError(err)
	return key
}

func LoadKeyPair(vanityName string) (key *rsa.PrivateKey) {
	dir := DATA_DIR + "/" + Context.ThisNodeAlias
	os.MkdirAll(dir, os.ModePerm)


	for i := 0; i < 1000; i++ {
		k1, _ := rsa.GenerateKey(rand.Reader, 2048)
		fmt.Println(k1.PublicKey)
	}

	vanityName = strings.ToUpper(vanityName)
	//.WithPadding(base32.NoPadding)
	vanityNameBin, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(vanityName)
	fmt.Println(vanityNameBin)
	if err != nil {
		fmt.Println(err.Error())
		fmt.Println("Warning: invalid vanity name specified (must be in Base32). It will be ignored.")
		vanityName = ""
		vanityNameBin = []byte{}
	}

	keyBin, err := ioutil.ReadFile(dir + "/key.bin")
	if err == nil {
		key = &rsa.PrivateKey{}
		decoder := gob.NewDecoder(bytes.NewBuffer(keyBin))
		err := decoder.Decode(key)
		FailOnError(err)
	} else {
		// Generate a new key
		key = GenerateKeyPair(vanityNameBin)
	}

	if !reflect.DeepEqual(ComputePublicKeyFingerprint(&key.PublicKey)[:len(vanityNameBin)], vanityNameBin) {
		fmt.Println("Warning: the public key does not respect the specified vanity name prefix.")
		fmt.Println("We suggest you to delete the current key and generate a new one.")
	}

	return
}

// ComputePublicKeyFingerprint computes the SHA-256 fingerprint of an RSA public key
func ComputePublicKeyFingerprint(publicKey *rsa.PublicKey) []byte {
	exponent := make([]byte, 4)
	binary.LittleEndian.PutUint32(exponent, uint32(publicKey.E))
	hash := sha256.Sum256(append(publicKey.N.Bytes(), exponent...))
	return hash[:]
}

// DeriveNameFromFingerprint generates a display name from the SHA-256 fingerprint
// of an RSA public key. The first 80 bits of the hash are converted to Base32,
// generating an alphanumeric string of 16 characters.
func DeriveNameFromFingerprint(hash []byte) string {
	return strings.ToLower(base32.StdEncoding.EncodeToString(hash[:10]))
}