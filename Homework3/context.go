package main

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"time"
)

// Classes for peers
const (
	Manual         = 0
	Learned        = 1
	ShortCircuited = 2
)

type contextType struct {
	EventQueue        chan func()
	GossipSocket      Socket
	PeerSet           map[string]int // The integer value represents the class
	Messages          map[string][]GossipMessageEntry
	ThisNodeName      string
	ThisNodeAddress   string
	MessageLog        []MessageLogEntry
	PrivateMessageLog map[string][]MessageLogEntry
	RoutingTable      map[string]string
	NoForward         bool
	DisableTraversal  bool
	SharedFiles       []*SharedFile
	ChunkDatabase	  map[string][]byte // Hash -> 8 KB chunk

	StatusSubscriptions   map[string]func(statusMessage *StatusPacket)
	DownloadSubscriptions map[string]func([]byte) // Hash -> functor
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
			fmt.Printf("DSDV %s: %s\n", origin, originAddress)

			return true, nil
		} else if id == expectedNextID-1 {
			// Already seen (last message)
			if !c.DisableTraversal && previousAddress == "" {
				// Direct route message -> override route
				c.RoutingTable[origin] = originAddress
				fmt.Printf("DIRECT-ROUTE FOR %s: %s\n", origin, originAddress)
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
			fmt.Printf("DSDV %s: %s\n", origin, originAddress)

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
			fmt.Printf("Not forwarding private message \"%s\" from %s to %s (noforward set)\n", msg.Text, msg.Origin, msg.Destination)
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
			} else {
				fmt.Printf("Not forwarding private message \"%s\" from %s to %s (no route found)\n", msg.Text, msg.Origin, msg.Destination)
			}
		} else {
			fmt.Printf("Not forwarding private message \"%s\" from %s to %s (TTL exceeded)\n", msg.Text, msg.Origin, msg.Destination)
		}
	}
	// In all other cases (hop limit reached, no route found), the message is discarded
}

func (c *contextType) ForwardDataRequest(msg *DataRequest) {
	if msg.Destination == c.ThisNodeName {
		// The request has reached its destination

		if msg.FileName != "" {
			// This is a metafile request
			file := c.GetFileByNameAndHash(msg.FileName, msg.HashValue)
			if file != nil {
				// File found -> send metafile to requester
				reply := &DataReply{c.ThisNodeName, msg.Origin, 10, msg.FileName, msg.HashValue, file.MetaFile}
				c.ForwardDataReply(reply)
			} else {
				// File not found -> send special message with empty metafile
				reply := &DataReply{c.ThisNodeName, msg.Origin, 10, msg.FileName, msg.HashValue, make([]byte, 0)}
				c.ForwardDataReply(reply)
			}
		} else {
			// This is a chunk request
			chunkData, found := c.ChunkDatabase[string(msg.HashValue)]
			if found {
				reply := &DataReply{c.ThisNodeName, msg.Origin, 10, "", msg.HashValue, chunkData}
				c.ForwardDataReply(reply)
			}
			// Else: drop the message silently
		}

	} else {
		if Context.NoForward {
			return
		}

		if msg.HopLimit > 0 {
			msg.HopLimit--

			// Find next hop
			next, found := c.RoutingTable[msg.Destination]
			if found {
				outMsg := GossipPacket{DataReq: msg}
				Context.GossipSocket.Send(Encode(&outMsg), next)
			}
		}
	}
	// In all other cases (hop limit reached, no route found), the packet is discarded
}

func (c *contextType) ForwardDataReply(msg *DataReply) {
	if msg.Destination == c.ThisNodeName {
		// The reply has reached its destination
		// Try to see if some task has requested a chunk/metafile with the given hash
		callback, found := c.DownloadSubscriptions[string(msg.HashValue)]
		if found {
			// Forward the data to the requester
			callback(msg.Data)
		}
		// Else: do nothing (i.e. drop the message)

	} else {
		if Context.NoForward {
			return
		}

		if msg.HopLimit > 0 {
			msg.HopLimit--

			// Find next hop
			next, found := c.RoutingTable[msg.Destination]
			if found {
				outMsg := GossipPacket{DataRep: msg}
				Context.GossipSocket.Send(Encode(&outMsg), next)
			}
		}
	}
	// In all other cases (hop limit reached, no route found), the packet is discarded
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

func (c *contextType) AddFile(name string, content []byte) *SharedFile {
	SaveFile(name, content)
	metadata := BuildMetadata(name, content)
	c.SharedFiles = append(c.SharedFiles, metadata)

	// Add chunks to lookup database
	numChunks := len(metadata.MetaFile)/32
	for i := 0; i < numChunks; i++ {
		chunkHash := metadata.MetaFile[i * 32 : (i+1)*32]
		start := i * CHUNK_SIZE     // Inclusive
		end := (i + 1) * CHUNK_SIZE // Exclusive
		if end > len(content) {
			end = len(content)
		}
		c.ChunkDatabase[string(chunkHash)] = content[start:end]
	}

	return metadata
}

func (c *contextType) GetFileByNameAndHash(fileName string, fileHash []byte) *SharedFile {
	for _, file := range c.SharedFiles {
		if file.FileName == fileName && reflect.DeepEqual(file.MetaHash, fileHash) {
			// Found it!
			return file
		}
	}
	return nil
}

func (c *contextType) FindOrRetrieveFile(fromPeer string, fileName string, fileHash []byte, callback func(*SharedFile, error)) {
	// Check if the file already exists in the local database. If this is the case, return it.
	if file := c.GetFileByNameAndHash(fileName, fileHash); file != nil {
		callback(file, nil)
		return
	}

	// File not found in local database -> download it from the given node
	if fromPeer != c.ThisNodeName {
		_, found := c.RoutingTable[fromPeer]
		if !found {
			callback(nil, errors.New("The given node does not exist"))
			return
		}
	} else {
		// Borderline case: the user has requested to download a file from THIS node, but the file does not exist
		callback(nil, errors.New("File not found in local database"))
		return
	}

	fmt.Printf("DOWNLOADING metafile of %s from %s\n", fileName, fromPeer)

	// Subscribe for replies having the given hash
	metaFile := make(chan []byte)
	c.DownloadSubscriptions[string(fileHash)] = func(data []byte) {
		metaFile <- data
	}

	// Run async task to wait for the metafile (or, upon timeout, resend the request)
	go func() {
		metaFileReceived := false
		retryCount := 0
		for !metaFileReceived {
			c.RunSync(func() {
				req := &DataRequest{c.ThisNodeName, fromPeer, 10, fileName, fileHash}
				c.ForwardDataRequest(req)
			})
			timeoutTimer := time.After(5 * time.Second)
			var result []byte
			select { // Whichever comes first (timeout or metafile)...
			case meta := <-metaFile:
				metaFileReceived = true
				result = meta
			case <-timeoutTimer:
					// Maybe we will have better luck at the next iteration
					retryCount++
			}
			if metaFileReceived {
				c.RunSync(func() {
					// Unsubscribe
					delete(c.DownloadSubscriptions, string(fileHash))
				})
				// An empty metafile means that the other side does not have the file
				if len(result) > 0 {
					c.DownloadFileFromPeer(fromPeer, fileName, result, func(fullFile *SharedFile) {
						if fullFile != nil {
							callback(fullFile, nil)
						} else {
							callback(nil, errors.New(
								"Connection with the destination lost (timed out after 10 retries)"))
						}
					});
				} else {
					// The node does not have the file (signaled through a nil data field)
					callback(nil, errors.New("The destination node does not have the file"))
					return
				}
			} else if retryCount == 3 {
				c.RunSync(func() {
					// Unsubscribe
					delete(c.DownloadSubscriptions, string(fileHash))
				})
				callback(nil, errors.New("The destination node does not answer (timed out after 3 retries)"))
				return
			}
		}
	}()
}

func (c* contextType) DownloadFileFromPeer(fromPeer string, fileName string, metaFile []byte, onCompletion func(*SharedFile)) {
	go func() {
		numChunks := len(metaFile) / 32
		reconstructedFile := make([]byte, 0)
		for i := 0; i < numChunks; i++ {
			fmt.Printf("DOWNLOADING %s chunk %d from %s\n", fileName, i + 1, fromPeer)
			chunkHash := metaFile[i*32 : (i+1)*32]

			chunkData := make(chan []byte)
			c.RunSync(func() {
				c.DownloadSubscriptions[string(chunkHash)] = func(data []byte) {
					chunkData <- data
				}
			})

			obtained := false
			// We keep a retry count, so that we stop the request if the destination does not answer within 10 retries
			retryCount := 0
			for !obtained {
				c.RunSync(func() {
					// Request next chunk
					request := &DataRequest{c.ThisNodeName, fromPeer, 10, "", chunkHash}
					c.ForwardDataRequest(request)
				})
				timeoutTimer := time.After(1 * time.Second) // Retransmit timeout
				select { // Whichever comes first (timeout or metafile)...
				case result := <-chunkData:
					retryCount = 0 // Data received -> reset the retry count
					reconstructedFile = append(reconstructedFile, result...)
					c.RunSync(func() {
						c.ChunkDatabase[string(chunkHash)] = result
					})
					obtained = true
				case <-timeoutTimer:
					retryCount++
					if retryCount == 10 {
						c.RunSync(func() {
							// Unsubscribe
							delete(c.DownloadSubscriptions, string(chunkHash))
						})
						onCompletion(nil)
						return
					}
					// Retransmit the request at the next iteration

				}
			}

			c.RunSync(func() {
				// Unsubscribe
				delete(c.DownloadSubscriptions, string(chunkHash))
			})
		}

		fmt.Printf("RECONSTRUCTED file %s\n", fileName)
		c.RunSync(func() {
			filePtr := c.AddFile(fileName, reconstructedFile)
			onCompletion(filePtr) // Finally, forward the full file to whomever requested it (e.g. web server)
		})
	}()
}

// RunSync runs a synchronous task on the main event loop, and waits until the task has finished
func (c* contextType) RunSync(event func()) {
	proceed := make(chan bool)
	c.EventQueue <- func() {
		event()
		proceed <- true
	}
	<- proceed
}

// InitializeFileDatabase must be called when the application starts up. It fetches the download directory
// and adds all files to the local database.
func (c *contextType) InitializeFileDatabase() {
	files := ListFiles()
	for _, fileName := range files {
		content, _ := LoadFile(fileName)
		c.AddFile(fileName, content)
	}
}