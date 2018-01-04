package main

import (
	"errors"
	"math/rand"
	"time"
	"strings"
)

// Classes for peers
const (
	Manual  = 0
	Learned = 1
)

type contextType struct {
	EventQueue      chan func()
	GossipSocket    Socket
	ThisNodeAddress string
	PeerSet         map[string]int // The integer value represents the class

	StatusSubscriptions map[string]func(statusMessage *StatusPacket)

	PrivateKey  PrivateKey
	PublicKey   PublicKey
	DisplayName string // Self-signing name of this node, derived from the public key

	Database *DbConnection

	PowTarget int // Number of leading zeros for proof-of-work
}

var Context contextType

// GetMyNextID returns the next ID of this node.
func (c *contextType) GetMyNextID() uint32 {
	return c.Database.NextID(c.DisplayName)
}

// AddNewMessage adds a new message to this gossiper (when received from a client) and returns its ID.
func (c *contextType) AddNewMessage(message string) uint32 {

	var m *MessageRecord
	var nextID uint32
	// Run on main thread
	c.RunSync(func() {
		nextID = c.GetMyNextID()

		m = &MessageRecord{}
		m.Data.ID = nextID
		m.Data.Origin = c.DisplayName
		m.Data.Destination = ""          // Public message
		m.Data.Content = []byte(message) // Unencrypted content (since it is public)
		m.Data.Signature = c.PrivateKey.Sign(m.Data.Payload())
		m.FromAddress = "localhost:" + strings.Split(c.ThisNodeAddress, ":")[1]
		m.DateSeen = time.Now().Format(time.RFC3339)
	})

	// Compute proof-of-work nonce on the caller thread
	m.Data.ComputeNonce(c.PowTarget)

	c.RunSync(func() {
		err := c.VerifyMessage(&m.Data)
		if err != nil {
			// Something wrong has happened
			panic(err)
		}

		c.Database.InsertOrUpdateMessage(m)
	})
	return nextID
}

// VerifyMessage verifies the content of a message prior to accepting it, in terms of its structure,
// proof-of-work nonce, and digital signature.
func (c *contextType) VerifyMessage(message *RumorMessage) error {
	// Verify the structure of the message (as well as the proof-of-work nonce)
	err := message.SanityCheck(c.PowTarget)
	if err != nil {
		return err
	}

	if message.ID == 0 {
		// This message represents a public key announcement. Let's verify it.
		// Note that we do not need the digital signature ("Signature" field) to validate the message,
		// as the names are self-signing (they are derived from the public key).
		pk, err := DeserializePublicKey(message.Content)
		if err != nil {
			return err
		}
		if pk.DeriveName() != message.Origin {
			return errors.New("invalid public key associated with " + message.Origin + " (verification failed)")
		}
	} else {
		// This is a regular message -> get the public key of the node
		pk, err := c.GetPublicKeyOf(message.Origin)
		if err != nil {
			// If the public key cannot be found, this message is either bogus/corrupt or out-of-order
			return err
		}
		if !pk.Verify(message.Payload(), message.Signature) {
			return errors.New("invalid digital signature (verification failed)")
		}
	}

	// All tests passed!
	return nil
}

// TryInsertMessage inserts a new message in order.
// Returns true if the message is inserted, and false if it was already seen.
// An error is returned if the supplied ID is not the expected next ID (i.e. if the message is out of order)
// Note that the message is assumed to have already been verified for correctness.
func (c *contextType) TryInsertMessage(m *RumorMessage, originAddress string) (bool, error) {
	expectedNextID := c.Database.NextID(m.Origin)
	if m.ID == expectedNextID {
		// New message (in order)
		mr := &MessageRecord{}
		mr.Data = *m
		mr.FromAddress = originAddress
		mr.DateSeen = time.Now().Format(time.RFC3339)
		c.Database.InsertOrUpdateMessage(mr)
		return true, nil

	} else if m.ID < expectedNextID {
		// Already seen.
		// In this case, conflicts are resolved by adopting the message with the lowest hash.
		// Note that messages are already verified at this point, so this case can happen only if the sender
		// tries to send different messages having the same ID (with possibly malicious intent).

		dbMsg := c.Database.GetMessage(m.Origin, m.ID).Data
		if CompareHashes(m.ComputeHash(), dbMsg.ComputeHash()) == -1 {
			// Replace the old message with the new one
			mr := &MessageRecord{}
			mr.Data = *m
			mr.FromAddress = originAddress
			mr.DateSeen = time.Now().Format(time.RFC3339)
			c.Database.InsertOrUpdateMessage(mr)
			return true, nil // We return true to redistribute the message
		}

		return false, nil
	} else {
		// Out-of-order delivery -> return an error
		return false, errors.New("out of order message")
	}
}

// BuildStatusMessage returns a status packet with the vector clock of all the messages seen so far by this node
func (c *contextType) BuildStatusMessage() *StatusPacket {
	status := &StatusPacket{}
	status.Want = c.Database.VectorClock()
	return status
}

// BuildRumorMessage returns a rumor message with the given (origin, ID) pair.
func (c *contextType) BuildRumorMessage(origin string, id uint32) *RumorMessage {
	return &c.Database.GetMessage(origin, id).Data
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
	this := c.Database.VectorClock()

	// Compare lengths first
	if len(this) != len(other) {
		return false
	}

	vcMap := make(map[string]uint32)
	for _, record := range this {
		vcMap[record.Identifier] = record.NextID
	}

	for _, otherStatus := range other {
		match, found := vcMap[otherStatus.Identifier]
		if found {
			if match != otherStatus.NextID {
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
	this := c.Database.VectorClock()
	vcMap := make(map[string]uint32)
	for _, record := range this {
		vcMap[record.Identifier] = record.NextID
	}

	// The difference is computed in linear time by using hash sets
	otherDiff := make([]PeerStatus, 0)
	thisDiff := make([]PeerStatus, 0)

	otherSet := make(map[string]bool)

	for _, otherStatus := range other {
		otherSet[otherStatus.Identifier] = true

		match, found := vcMap[otherStatus.Identifier]
		if found {
			if match > otherStatus.NextID {
				otherDiff = append(otherDiff, otherStatus)
			} else if match < otherStatus.NextID {
				thisDiff = append(thisDiff, PeerStatus{otherStatus.Identifier, match})
			}
		} else if otherStatus.NextID > 0 {
			thisDiff = append(thisDiff, PeerStatus{otherStatus.Identifier, match})
		}
	}

	for peerName, _ := range vcMap {
		if !otherSet[peerName] {
			otherDiff = append(otherDiff, PeerStatus{peerName, 0})
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

func (c *contextType) GetPublicKeyOf(node string) (PublicKey, error) {
	// Get special message with ID = 0 (public key announcement)
	msg := c.Database.GetMessage(node, 0)
	if msg == nil {
		// Unknown node
		return nil, errors.New("public key of node not found in database (unknown node)")
	}

	pk, err := DeserializePublicKey(msg.Data.Content)
	if err != nil {
		return nil, errors.New("unable to decode the public key of the node (database corrupted?)")
	}
	return pk, nil
}

// InsertKeyAnnouncementMessage adds a new message with the public key announcement of THIS node
func (c *contextType) InsertKeyAnnouncementMessage() {
	nextID := c.GetMyNextID()
	if nextID == 0 {
		m := &MessageRecord{}
		m.Data.ID = nextID
		m.Data.Origin = c.DisplayName
		m.Data.Destination = ""                  // Public message
		m.Data.Content = c.PublicKey.Serialize() // The content is our public key (serialized to bytes)
		m.Data.Signature = make([]byte, 0)       // Not needed, since the name is self-signing
		m.Data.ComputeNonce(c.PowTarget)
		m.FromAddress = "localhost:" + strings.Split(c.ThisNodeAddress, ":")[1]
		m.DateSeen = time.Now().Format(time.RFC3339)

		c.Database.InsertOrUpdateMessage(m)
	}
}
