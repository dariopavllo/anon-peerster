package main

import (
	"github.com/dedis/protobuf"
	"net"
)

type RumorMessage struct {
	Origin   string
	ID       uint32
	Text     string
	LastIP   *net.IP
	LastPort *int
}

type PeerStatus struct {
	Identifier string
	NextID     uint32
}

type StatusPacket struct {
	Want []PeerStatus
}

type GossipPacket struct {
	Rumor     *RumorMessage
	Status    *StatusPacket
}

func Decode(data []byte, message interface{}) error {
	return protobuf.Decode(data, message)
}

func Encode(message interface{}) []byte {
	encoded, err := protobuf.Encode(message)
	FailOnError(err)
	return encoded
}
