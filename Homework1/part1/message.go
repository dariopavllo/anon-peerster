package main

import (
	"github.com/dedis/protobuf"
)

// ClientMessage represents a message exchange between a client and a peer
type ClientMessage struct {
	Text string
}

// PeerMessage represents a message exchanged between two peers
type PeerMessage struct {
	OriginalSenderName string
	RelayPeerAddress   string
	Text               string
}

func Decode(data []byte, message interface{}) error {
	return protobuf.Decode(data, message)
}

func Encode(message interface{}) []byte {
	encoded, err := protobuf.Encode(message)
	FailOnError(err)
	return encoded
}
