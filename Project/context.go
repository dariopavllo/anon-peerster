package main

import (
	"crypto/rsa"
	"errors"
	"math/rand"
	"time"
)

// Classes for peers
const (
	Manual  = 0
	Learned = 1
)

type contextType struct {
	EventQueue      chan func()
	GossipSocket    Socket
	PeerSet         map[string]int // The integer value represents the class
	Messages        map[string][]GossipMessageEntry
	ThisNodeAlias   string
	ThisNodeAddress string
	MessageLog      []MessageLogEntry

	StatusSubscriptions map[string]func(statusMessage *StatusPacket)

	PrivateKey  *rsa.PrivateKey
	PublicKey   *rsa.PublicKey
	DisplayName string
}

type MessageLogEntry struct {
	FirstSeen   time.Time
	FromNode    string
	SeqID       uint32
	FromAddress string
	Content     string
}

type GossipMessageEntry struct {
	LastSender string
	Text       string
}

var Context contextType

// GetMyNextID returns the next ID of this node.
func (c *contextType) GetMyNextID() uint32 {
	messages, found := c.Messages[c.DisplayName]
	var nextID int
	if found {
		nextID = len(messages) + 1
	} else {
		nextID = 1
	}
	return uint32(nextID)
}

// AddNewMessage adds a new message to this gossiper (when received from a client) and returns its ID.
func (c *contextType) AddNewMessage(message string) uint32 {
	nextID := c.GetMyNextID()
	if nextID == 1 {
		// Initialize structure on the first message
		c.Messages[c.DisplayName] = make([]GossipMessageEntry, 0)
	}

	c.Messages[c.DisplayName] = append(c.Messages[c.DisplayName], GossipMessageEntry{"", message})

	c.MessageLog = append(c.MessageLog,
		MessageLogEntry{time.Now(), c.DisplayName, nextID, c.ThisNodeAddress, message})
	return nextID
}

// TryInsertMessage inserts a new message in order.
// Returns true if the message is inserted, and false if it was already seen.
// An error is returned if the supplied ID is not the expected next ID (i.e. if the message is out of order)
func (c *contextType) TryInsertMessage(origin string, originAddress string, message string, id uint32, previousAddress string) (bool, error) {
	if id == 0 {
		return false, errors.New("message IDs must start from 1")
	}

	messages, found := c.Messages[origin]
	if found {
		expectedNextID := uint32(len(messages) + 1)
		if id == expectedNextID {
			// New message (in order)
			c.Messages[origin] = append(c.Messages[origin], GossipMessageEntry{originAddress, message})

			c.MessageLog = append(c.MessageLog,
				MessageLogEntry{time.Now(), origin, id, originAddress, message})

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
			c.Messages[origin] = make([]GossipMessageEntry, 1)
			c.Messages[origin][0] = GossipMessageEntry{originAddress, message}
			c.MessageLog = append(c.MessageLog,
				MessageLogEntry{time.Now(), origin, 1, originAddress, message})

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
	rumor.Text = c.Messages[origin][id-1].Text
	rumor.ID = id
	rumor.LastIP, rumor.LastPort = SplitAddress(c.Messages[origin][id-1].LastSender)
	return rumor
}

// RandomPeer selects a random peer from the current set of peers.
// exclusionList defines the set of peers to be excluded from the selection.
// If no valid peer can be found, an empty string is returned.
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
// The first return value represents the messages seen by this node (but not by the other node),
// whereas the second return value represents the messages seen by the other  node, but not by this node.
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
		} else if otherStatus.NextID > 1 {
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

// SendStatusMessage sends a status message to the given peer.
func (c *contextType) SendStatusMessage(peerAddress string) {
	statusMsg := c.BuildStatusMessage()
	gossipMsg := GossipPacket{Status: statusMsg}
	Context.GossipSocket.Send(Encode(&gossipMsg), peerAddress)
}

// RunSync runs a synchronous task on the main event loop, and waits until the task has finished
func (c *contextType) RunSync(event func()) {
	proceed := make(chan bool)
	c.EventQueue <- func() {
		event()
		proceed <- true
	}
	<-proceed
}
