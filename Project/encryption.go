package main

import (
	"crypto/sha256"
	"encoding/binary"
	"crypto/rsa"
	"crypto/rand"
	"encoding/base32"
	"strings"
)

// GenerateKeyPair generates a 2048-bit RSA public/private key pair
func GenerateKeyPair() {
	rsa.GenerateKey(rand.Reader, 2048)
}

func LoadKeyPair() {

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