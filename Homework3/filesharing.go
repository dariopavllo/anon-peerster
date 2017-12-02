package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
)

type SharedFile struct {
	FileName string
	FileSize int
	MetaFile []byte // Meta file (up to 8 KB in size)
	MetaHash []byte // Hash of the metafile
}

const DOWNLOAD_DIR = "_Downloads"
const CHUNK_SIZE = 8192

func BuildMetadata(name string, content []byte) *SharedFile {
	numChunks := (len(content) + CHUNK_SIZE - 1) / CHUNK_SIZE // Integer round up
	chunkHashes := make([]byte, sha256.Size*numChunks)
	for i := 0; i < numChunks; i++ {
		start := i * CHUNK_SIZE     // Inclusive
		end := (i + 1) * CHUNK_SIZE // Exclusive
		if end > len(content) {
			end = len(content)
		}

		chunk := content[start:end]
		hash := sha256.Sum256(chunk)
		copy(chunkHashes[i*sha256.Size:(i+1)*sha256.Size], hash[:])
	}

	totalHash := sha256.Sum256(chunkHashes)
	fmt.Printf("Hash for %s: %s\n", name, hex.EncodeToString(totalHash[:]))
	return &SharedFile{name, len(content), chunkHashes, totalHash[:]}
}

// VerifyMetafile verifies whether a received metaFile is valid
func VerifyMetafile(metaHash []byte, metaFile []byte) bool {
	if len(metaFile) == 0 {
		// An empty metafile is ok: it means that the sender does not have the requested file
		return true
	}
	// First check: the length of the metaFile must be a multiple of 32 (SHA-256 size in bytes)
	if len(metaFile) % sha256.Size != 0 {
		return false
	}

	// Second check: verify the hash
	actual := sha256.Sum256(metaFile)
	return reflect.DeepEqual(metaHash, actual[:])
}

// VerifyMetafile verifies whether received chunk is valid
func VerifyChunk(chunkIndex int, metaFile []byte, receivedData []byte) bool {
	expected := metaFile[chunkIndex * sha256.Size : (chunkIndex + 1) * sha256.Size]
	actual := sha256.Sum256(receivedData)
	return reflect.DeepEqual(expected, actual[:])
}

func SaveFile(name string, content []byte) {
	dir := DOWNLOAD_DIR + "/" + Context.ThisNodeName
	os.MkdirAll(dir, os.ModePerm)
	ioutil.WriteFile(dir+"/"+name, content, 0644)
}

func LoadFile(name string) ([]byte, error) {
	dir := DOWNLOAD_DIR + "/" + Context.ThisNodeName
	return ioutil.ReadFile(dir + "/" + name)
}

func ListFiles() []string {
	dir := DOWNLOAD_DIR + "/" + Context.ThisNodeName
	os.MkdirAll(dir, os.ModePerm)
	files, _ := ioutil.ReadDir(dir)
	result := make([]string, len(files))
	for i, file := range files {
		result[i] = file.Name()
	}
	return result
}