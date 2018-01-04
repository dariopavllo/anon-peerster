package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/dedis/protobuf"
	"strings"
	"time"
)

// Proof-of-work nonce length (in bytes)
const NONCE_LENGTH = 16

type RumorMessage struct {
	Origin      string
	Destination string
	ID          uint32
	Content     []byte
	Signature   []byte
	Nonce       []byte
}

type PeerStatus struct {
	Identifier string
	NextID     uint32
}

type StatusPacket struct {
	Want []PeerStatus
}

type GossipPacket struct {
	Rumor  *RumorMessage
	Status *StatusPacket
}

func Decode(data []byte, message interface{}) error {
	return protobuf.Decode(data, message)
}

func Encode(message interface{}) []byte {
	encoded, err := protobuf.Encode(message)
	FailOnError(err)
	return encoded
}

func (m *RumorMessage) SanityCheck(powTarget int) error {
	if len(m.Origin) != DISPLAY_NAME_BITS/5 {
		return errors.New("invalid origin length")
	}

	// An empty destination (length = 0) means that a message is delivered to everyone (as in key announcements)
	if len(m.Destination) != DISPLAY_NAME_BITS/5 && len(m.Destination) != 0 {
		return errors.New("invalid destination length")
	}

	if len(m.Nonce) != NONCE_LENGTH {
		return errors.New("invalid nonce length")
	}

	_, err := base32.StdEncoding.DecodeString(strings.ToUpper(m.Origin))
	if err != nil {
		return errors.New("invalid origin")
	}

	if len(m.Destination) != 0 {
		_, err := base32.StdEncoding.DecodeString(strings.ToUpper(m.Destination))
		if err != nil {
			return errors.New("invalid destination")
		}
	}

	if NumLeadingZeros(m.ComputeHash()) < powTarget {
		return errors.New("invalid nonce (not enough leading zeros)")
	}

	return nil
}

// ComputeHash returns the SHA-256 hash of this message (calculated on all fields)
func (m *RumorMessage) ComputeHash() []byte {
	hash := sha256.New()
	hash.Write([]byte(m.Origin))
	hash.Write([]byte(m.Destination))
	id := make([]byte, 4)
	binary.LittleEndian.PutUint32(id, uint32(m.ID))
	hash.Write(id)
	hash.Write(m.Content)
	hash.Write(m.Signature)
	hash.Write(m.Nonce)
	return hash.Sum(nil)
}

// Payload returns the actual contents of this message (ID, origin, destination, message).
// This method is typically used for signing the message.
func (m *RumorMessage) Payload() []byte {
	var b bytes.Buffer
	b.Write([]byte(m.Origin))
	b.Write([]byte(m.Destination))
	id := make([]byte, 4)
	binary.LittleEndian.PutUint32(id, uint32(m.ID))
	b.Write(id)
	b.Write(m.Content)
	return b.Bytes()
}

// ComputeNonce computes the proof-of-work nonce for this message, according to the given target (number of leading zeros).
// The process may require a long time, since the nonce is bruteforced.
func (m *RumorMessage) ComputeNonce(target int) {
	// The initial nonce is "all zeros"
	m.Nonce = make([]byte, NONCE_LENGTH)

	fmt.Printf("Starting a nonce computation with %d leading zeros...\n", target)
	t1 := time.Now()
	tries := uint64(0)
	for {
		tries++
		if NumLeadingZeros(m.ComputeHash()) >= target {
			break
		}

		// Increment the nonce by 1
		for i := 0; i < NONCE_LENGTH; i++ {
			if m.Nonce[i] < 255 {
				m.Nonce[i]++
				break
			} else {
				m.Nonce[i] = 0
			}
		}
	}
	t2 := time.Now()
	fmt.Printf("Nonce computed in %.2f seconds (%d tries)\n", t2.Sub(t1).Seconds(), tries)
}
