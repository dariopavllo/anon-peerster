package main

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
)

// The size of the RSA private key, in bits
const RSA_KEY_SIZE_BITS = 2048

// Length of the display name (in bits), extracted from the SHA-256 fingerprint of the public key
const DISPLAY_NAME_BITS = 80

type PublicKey interface {
	Verify(message []byte, signature []byte) bool
	Encrypt(message []byte) []byte
	Serialize() []byte
	Fingerprint() []byte
	DeriveName() string
}

type PrivateKey interface {
	Sign(message []byte) []byte
	Decrypt(ciphertext []byte) ([]byte, error)
}

type RsaPublicKey struct {
	key *rsa.PublicKey
}

type RsaPrivateKey struct {
	key *rsa.PrivateKey
}

// GenerateKeyPair generates a 2048-bit RSA public/private key pair
func GenerateKeyPair(dataDirectory string) (PrivateKey, PublicKey) {
	fmt.Println("Generating a 2048-bit RSA keypair for the first time.")

	key, err := rsa.GenerateKey(rand.Reader, RSA_KEY_SIZE_BITS)
	FailOnError(err)

	// Save the key on file
	os.MkdirAll(dataDirectory, os.ModePerm)

	buf := bytes.Buffer{}
	encoder := gob.NewEncoder(&buf)
	err = encoder.Encode(key)
	FailOnError(err)

	err = ioutil.WriteFile(dataDirectory+"/key.bin", buf.Bytes(), 0644)
	FailOnError(err)
	return &RsaPrivateKey{key}, &RsaPublicKey{&key.PublicKey}
}

func LoadKeyPair(dataDirectory string) (PrivateKey, PublicKey) {
	os.MkdirAll(dataDirectory, os.ModePerm)

	keyBin, err := ioutil.ReadFile(dataDirectory + "/key.bin")
	var key *rsa.PrivateKey
	if err == nil {
		key = &rsa.PrivateKey{}
		decoder := gob.NewDecoder(bytes.NewBuffer(keyBin))
		err := decoder.Decode(key)
		FailOnError(err)
		FailOnError(key.Validate())
		return &RsaPrivateKey{key}, &RsaPublicKey{&key.PublicKey}
	} else {
		// Generate a new key
		return GenerateKeyPair(dataDirectory)
	}
}

func (k *RsaPublicKey) Serialize() []byte {
	exponent := make([]byte, 4)
	binary.LittleEndian.PutUint32(exponent, uint32(k.key.E))
	return append(k.key.N.Bytes(), exponent...)
}

func DeserializePublicKey(data []byte) (PublicKey, error) {
	if len(data) != RSA_KEY_SIZE_BITS/8+4 {
		return nil, errors.New("invalid key length")
	}
	pkLen := len(data) - 4
	publicKey := &rsa.PublicKey{}
	publicKey.N = big.NewInt(0)
	publicKey.N.SetBytes(data[:pkLen])
	publicKey.E = int(binary.LittleEndian.Uint32(data[pkLen:]))
	return &RsaPublicKey{publicKey}, nil
}

// Fingerprint computes the SHA-256 fingerprint of an RSA public key
func (k *RsaPublicKey) Fingerprint() []byte {
	binaryKey := k.Serialize()
	hash := sha256.Sum256(binaryKey)
	return hash[:]
}

// DeriveName generates a display name from the SHA-256 fingerprint
// of an RSA public key. The first 80 bits of the hash are converted to Base32,
// generating an alphanumeric string of 16 characters.
func (k *RsaPublicKey) DeriveName() string {
	return strings.ToLower(base32.StdEncoding.EncodeToString(
		k.Fingerprint()[:DISPLAY_NAME_BITS/8]))
}

func (k *RsaPublicKey) Verify(message []byte, signature []byte) bool {
	digest := sha256.Sum256(message)
	err := rsa.VerifyPSS(k.key, crypto.SHA256, digest[:], signature, nil)
	return err == nil
}

func (k *RsaPublicKey) Encrypt(message []byte) []byte {
	enc, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, k.key, message, nil)
	FailOnError(err)
	return enc
}

func (k *RsaPrivateKey) Sign(message []byte) []byte {
	digest := sha256.Sum256(message)
	signature, err := rsa.SignPSS(rand.Reader, k.key, crypto.SHA256, digest[:], nil)
	FailOnError(err)
	return signature
}

func (k *RsaPrivateKey) Decrypt(ciphertext []byte) ([]byte, error) {
	return rsa.DecryptOAEP(sha256.New(), rand.Reader, k.key, ciphertext, nil)
}
