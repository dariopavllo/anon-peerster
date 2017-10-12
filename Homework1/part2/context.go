package main

import (
	"errors"
	"math/rand"
)

type contextType struct {
	EventQueue      chan func()
	GossipSocket    Socket
	PeerSet         map[string]bool
	Messages        map[string][]string
	ThisNodeName    string
	ThisNodeAddress string

	StatusSubscriptions map[string]func(statusMessage *StatusPacket)
}

var Context contextType

// AddNewMessage adds a new message to this gossiper (when received from a client) and returns its ID.
func (c *contextType) AddNewMessage(message string) uint32 {
	messages, found := c.Messages[c.ThisNodeName]
	var nextID int
	if found {
		nextID = len(messages) + 1
	} else {
		nextID = 1
		c.Messages[c.ThisNodeName] = make([]string, 0)
	}

	c.Messages[c.ThisNodeName] = append(c.Messages[c.ThisNodeName], message)
	return uint32(nextID)
}

// TryInsertMessage inserts a new message in order.
// Returns true if the message is inserted, and false if it was already seen.
// An error is returned if the supplied ID is not the expected next ID (i.e. if the message is out of order)
func (c *contextType) TryInsertMessage(origin string, message string, id uint32) (bool, error) {
	if id == 0 {
		return false, errors.New("message IDs must start from 1")
	}

	messages, found := c.Messages[origin]
	if found {
		expectedNextID := uint32(len(messages) + 1)
		if id == expectedNextID {
			// New message (in order)
			c.Messages[origin] = append(c.Messages[origin], message)
			return true, nil
		} else if id < expectedNextID {
			// Already seen
			return false, nil
		}

		// Out-of-order delivery -> return an error
		return false, errors.New("out of order message")
	} else {
		// First message from that origin
		if id == 1 {
			c.Messages[origin] = make([]string, 1)
			c.Messages[origin][0] = message
			return true, nil
		}

		// Out-of-order delivery -> return an error
		return false, errors.New("out of order message")
	}
}

// BuildStatusMessage returns a status packet with the vector clock of all the messages seen so far by this node
func (c *contextType) BuildStatusMessage() *StatusPacket {
	status := &StatusPacket{}
	status.Want = make([]PeerStatus, len(c.Messages))
	i := 0
	for name, messages := range c.Messages {
		status.Want[i].Identifier = name
		status.Want[i].NextID = uint32(len(messages) + 1)
		i++
	}
	return status
}

// BuildRumorMessage returns a rumor message with the given (origin, ID) pair.
func (c *contextType) BuildRumorMessage(origin string, id uint32) *RumorMessage {
	rumor := &RumorMessage{Origin: origin}
	rumor.PeerMessage.Text = c.Messages[origin][id-1]
	rumor.PeerMessage.ID = id
	return rumor
}

func (c *contextType) RandomPeer(exclusionList []string) string {
	validPeers := make([]string, 0)
	for peer := range c.PeerSet {
		if !IsInArray(peer, exclusionList) {
			validPeers = append(validPeers, peer)
		}
	}

	if len(validPeers) == 0 {
		return ""
	}

	return validPeers[rand.Intn(len(validPeers))]
}

// VectorClockEquals tells whether the vector clock of this node equals the vector clock of the other node.
func (c *contextType) VectorClockEquals(other []PeerStatus) bool {

	// Compare lengths first
	if len(c.Messages) != len(other) {
		return false
	}

	for _, otherStatus := range other {
		messages, found := c.Messages[otherStatus.Identifier]
		if found {
			if uint32(len(messages)+1) != otherStatus.NextID {
				return false
			}
		} else {
			return false
		}
	}

	return true
}

// VectorClockDifference returns the difference between two vector clocks.
// The first argument represents the messages seen by this node (but not by the other node),
// whereas the second argument represents the messages seen by the other  node, but not by this node.
func (c *contextType) VectorClockDifference(other []PeerStatus) ([]PeerStatus, []PeerStatus) {

	// The difference is computed in linear time by using hash sets
	otherDiff := make([]PeerStatus, 0)
	thisDiff := make([]PeerStatus, 0)

	otherSet := make(map[string]bool)

	for _, otherStatus := range other {
		otherSet[otherStatus.Identifier] = true

		messages, found := c.Messages[otherStatus.Identifier]
		if found {
			if uint32(len(messages)+1) > otherStatus.NextID {
				otherDiff = append(otherDiff, otherStatus)
			} else if uint32(len(messages)+1) < otherStatus.NextID {
				thisDiff = append(thisDiff, PeerStatus{otherStatus.Identifier, uint32(len(messages) + 1)})
			}
		} else {
			thisDiff = append(thisDiff, PeerStatus{otherStatus.Identifier, uint32(len(messages) + 1)})
		}
	}

	for peerName, _ := range c.Messages {
		if !otherSet[peerName] {
			otherDiff = append(otherDiff, PeerStatus{peerName, 1})
		}
	}

	return otherDiff, thisDiff
}

func (c *contextType) SendStatusMessage(peerAddress string) {
	statusMsg := c.BuildStatusMessage()
	gossipMsg := GossipPacket{Status: statusMsg}
	Context.GossipSocket.Send(Encode(&gossipMsg), peerAddress)
}
