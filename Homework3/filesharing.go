package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
)

type SharedFile struct {
	FileName string
	FileSize int
	MetaFile []byte
	MetaHash []byte
}

func BuildMetadata(name string, content []byte) *SharedFile {
	chunkSize := 8192
	numChunks := (len(content) + chunkSize - 1) / chunkSize // Integer round up
	chunkHashes := make([]byte, sha256.Size*numChunks)
	for i := 0; i < numChunks; i++ {
		start := i * chunkSize     // Inclusive
		end := (i + 1) * chunkSize // Exclusive
		if end > len(content) {
			end = len(content)
		}

		chunk := content[start:end]
		hash := sha256.Sum256(chunk)
		copy(chunkHashes[i*sha256.Size:(i+1)*sha256.Size], hash[:])
	}

	totalHash := sha256.Sum256(chunkHashes)
	fmt.Printf("Hash: %s\n", hex.EncodeToString(totalHash[:]))
	return &SharedFile{name, len(content), chunkHashes, totalHash[:]}
}

func SaveFile(name string, content []byte) {
	dir := "_Downloads/" + Context.ThisNodeName
	os.MkdirAll(dir, os.ModePerm)
	ioutil.WriteFile(dir+"/"+name, content, 0644)
}

func LoadFile(name string) ([]byte, error) {
	dir := "_Downloads/" + Context.ThisNodeName
	return ioutil.ReadFile(dir + "/" + name)
}
