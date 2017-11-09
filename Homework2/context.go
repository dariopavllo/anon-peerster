package main

import (
	"errors"
	"math/rand"
	"time"
	"fmt"
)

// Classes for peers
const (
	Manual = 0
	Learned = 1
	ShortCircuited = 2
)

type contextType struct {
	EventQueue      	chan func()
	GossipSocket    	Socket
	PeerSet         	map[string]int // The integer value represents the class
	Messages        	map[string][]GossipMessageEntry
	ThisNodeName    	string
	ThisNodeAddress 	string
	MessageLog      	[]MessageLogEntry
	PrivateMessageLog   map[string][]MessageLogEntry
	RoutingTable    	map[string]string
	NoForward			bool
	DisableTraversal	bool

	StatusSubscriptions map[string]func(statusMessage *StatusPacket)
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
	Text string
}

var Context contextType

// GetMyNextID returns the next ID of this node.
func (c *contextType) GetMyNextID() uint32 {
	messages, found := c.Messages[c.ThisNodeName]
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
		c.Messages[c.ThisNodeName] = make([]GossipMessageEntry, 0)
	}

	c.Messages[c.ThisNodeName] = append(c.Messages[c.ThisNodeName], GossipMessageEntry{"", message})

	// Display the message to the user only if it is not a route message
	if message != "" {
		c.MessageLog = append(c.MessageLog,
			MessageLogEntry{time.Now(), c.ThisNodeName, nextID, c.ThisNodeAddress, message})
	}
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

			// Display the message to the user only if it is not a route message
			if message != "" {
				c.MessageLog = append(c.MessageLog,
					MessageLogEntry{time.Now(), origin, id, originAddress, message})
			}

			// Update route unconditionally
			c.RoutingTable[origin] = originAddress

			return true, nil
		} else if id == expectedNextID - 1 {
			// Already seen (last message)
			if !c.DisableTraversal && previousAddress == "" {
				// Direct route message -> override route
				c.RoutingTable[origin] = originAddress
			}
			return false, nil
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
			// Display the message to the user only if it is not a route message
			if message != "" {
				c.MessageLog = append(c.MessageLog,
					MessageLogEntry{time.Now(), origin, 1, originAddress, message})
			}

			// Add route
			c.RoutingTable[origin] = originAddress
			fmt.Printf("DSDV %s:%s\n", origin, originAddress)

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

// BuildPrivateMessage returns a private message with the given destination and content.
func (c *contextType) BuildPrivateMessage(destinationName string, message string) *PrivateMessage {
	private := &PrivateMessage{Origin: c.ThisNodeName, Destination: destinationName, HopLimit: 10}
	private.Text = message
	private.ID = 0
	return private
}

func (c *contextType) ForwardPrivateMessage(sender string, msg *PrivateMessage) {
	if msg.Destination == c.ThisNodeName {
		// The message has reached its destination
		fmt.Printf("PRIVATE: %s:%d:%s", msg.Origin, msg.HopLimit, msg.Text)
		c.LogPrivateMessage(sender, msg)
	} else {
		if Context.NoForward {
			return
		}

		if msg.HopLimit > 0 {
			msg.HopLimit--

			// Find next hop
			next, found := c.RoutingTable[msg.Destination]
			if found {
				outMsg := GossipPacket{Private: msg}
				fmt.Printf("PRIVATE FORWARD \"%s\" from %s to %s\n", msg.Text, msg.Origin, next)
				Context.GossipSocket.Send(Encode(&outMsg), next)
			}
		}
	}
	// In all other cases (hop limit reached, no route found), the message is discarded
}

func (c *contextType) LogPrivateMessage(sender string, msg *PrivateMessage) {
	var targetName string
	if msg.Origin == c.ThisNodeName {
		targetName = msg.Destination
	} else {
		targetName = msg.Origin
	}

	_, found := c.PrivateMessageLog[targetName]
	if !found {
		c.PrivateMessageLog[targetName] = make([]MessageLogEntry, 0)
	}

	entry := MessageLogEntry{time.Now(), msg.Origin, 0, sender, msg.Text}
	c.PrivateMessageLog[targetName] = append(c.PrivateMessageLog[targetName], entry)
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

// BroadcastRoutes creates a new route rumor message and broadcasts it to all peers.
func (c *contextType) BroadcastRoutes() {
	id := c.AddNewMessage("")
	for peerAddress := range c.PeerSet {
		rumor := c.BuildRumorMessage(c.ThisNodeName, id)
		outMsg := GossipPacket{Rumor: rumor}
		c.GossipSocket.Send(Encode(&outMsg), peerAddress)
	}
}