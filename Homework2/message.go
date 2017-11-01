package main

import (
	"github.com/dedis/protobuf"
)

// ClientMessage represents a message exchanged between a CLI client and a peer
type ClientMessage struct {
	Text string
}

type RumorMessage struct {
	Origin      string
	PeerMessage struct {
		ID   uint32
		Text string
	}
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
