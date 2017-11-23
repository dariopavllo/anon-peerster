package main

import (
	"github.com/dedis/protobuf"
	"net"
)

// ClientMessage represents a message exchanged between a CLI client and a peer
type ClientMessage struct {
	Text string
}

type RumorMessage struct {
	Origin		string
	ID			uint32
	Text		string
	LastIP		*net.IP
	LastPort	*int
}

func (m *RumorMessage) IsRouteMessage() bool {
	return m.Text == ""
}

type PrivateMessage struct {
	Origin      string
	ID			uint32
	Text		string
	Destination	string
	HopLimit    uint32
}

type PeerStatus struct {
	Identifier string
	NextID     uint32
}

type StatusPacket struct {
	Want []PeerStatus
}

type GossipPacket struct {
	Rumor   *RumorMessage
	Status  *StatusPacket
	Private *PrivateMessage
}

func Decode(data []byte, message interface{}) error {
	return protobuf.Decode(data, message)
}

func Encode(message interface{}) []byte {
	encoded, err := protobuf.Encode(message)
	FailOnError(err)
	return encoded
}
