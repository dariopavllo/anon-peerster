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

func (m *RumorMessage) IsRouteMessage() bool {
	return m.Text == ""
}

type PrivateMessage struct {
	Origin      string
	ID          uint32
	Text        string
	Destination string
	HopLimit    uint32
}

type DataRequest struct {
	Origin      string
	Destination string
	HopLimit    uint32
	FileName    string
	HashValue   []byte
}

type DataReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	FileName    string
	HashValue   []byte
	Data        []byte
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
	DataReq *DataRequest
	DataRep *DataReply
}

func Decode(data []byte, message interface{}) error {
	return protobuf.Decode(data, message)
}

func Encode(message interface{}) []byte {
	encoded, err := protobuf.Encode(message)
	FailOnError(err)
	return encoded
}
