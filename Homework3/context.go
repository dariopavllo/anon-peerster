package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
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

	SharedFiles      []*SharedFile
	MetafileDatabase map[string]*FileDescriptor // Metahash -> (metafile, filename)
	ChunkDatabase    map[string][]byte         // Chunk hash -> 8 KB chunk
	PendingRequests  []*SearchRequest
	SearchResults    []*SearchReply // Search results for the last query

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

func (c *contextType) ForwardSearchRequest(sender string, msg *SearchRequest) {
	if msg.Budget == 0 {
		return
	}

	// Check if this request is a duplicate within 0.5 seconds -> if so, drop it
	for _, req := range c.PendingRequests {
		if req.Origin == msg.Origin && reflect.DeepEqual(req.Keywords, msg.Keywords) {
			return
		}
	}

	// The request is not a duplicate -> add it to the list and start an async job for destroying it after 0.5 seconds
	c.PendingRequests = append(c.PendingRequests, msg)
	time.AfterFunc(time.Millisecond*500, func() {
		Context.EventQueue <- func() {
			for i, req := range c.PendingRequests {
				if req.Origin == msg.Origin && reflect.DeepEqual(req.Keywords, msg.Keywords) {
					c.PendingRequests[i] = c.PendingRequests[len(c.PendingRequests)-1]
					c.PendingRequests = c.PendingRequests[:len(c.PendingRequests)-1]
					return
				}
			}
		}
	})

	if msg.Origin != c.ThisNodeName {
		// Local search
		searchResults := make([]*SearchResult, 0)
		for metaHash, file := range c.MetafileDatabase {
			for _, keyword := range msg.Keywords {
				if strings.Contains(file.FileName, keyword) {
					record := &SearchResult{file.FileName, []byte(metaHash), make([]uint64, 0)}

					// Enumerate the chunks available so far at this node
					numChunks := len(file.MetaFile) / 32
					for i := 0; i < numChunks; i++ {
						chunkHash := file.MetaFile[i*32 : (i+1)*32]
						if _, found := c.ChunkDatabase[string(chunkHash)]; found {
							record.ChunkMap = append(record.ChunkMap, uint64(i+1)) // 1-based chunk index
						}
					}
					searchResults = append(searchResults, record)
					break // Break the outer loop, so that we do not risk adding the same file twice
				}
			}
		}

		if len(searchResults) > 0 {
			// Some matches have been found -> send reply
			reply := &SearchReply{c.ThisNodeName, msg.Origin, 10, searchResults}
			c.ForwardSearchReply(reply)
		}

		msg.Budget--
	}

	// Do not forward the request if the budget has reached zero
	if msg.Budget == 0 {
		return
	}

	// Repartition the remaining budget among peers (excluding the last sender)
	nextPeers := make([]string, 0)
	for peer := range c.PeerSet {
		if peer != sender {
			nextPeers = append(nextPeers, peer)
		}
	}
	permutationList := rand.Perm(len(nextPeers))
	budgetAllocation := make([]int, len(nextPeers))
	i := 0
	for len(nextPeers) > 0 && msg.Budget > 0 {
		budgetAllocation[permutationList[i]]++
		msg.Budget--
		i = (i + 1) % len(nextPeers)
	}

	// Forward the request to the "lucky" peers
	for index, peer := range nextPeers {
		if budgetAllocation[index] > 0 {
			msg.Budget = uint64(budgetAllocation[index])
			outMsg := GossipPacket{SearchReq: msg}
			Context.GossipSocket.Send(Encode(&outMsg), peer)
		}
	}
}

func (c *contextType) ForwardSearchReply(msg *SearchReply) {
	if msg.Destination == c.ThisNodeName {
		// The reply has reached its destination

		// Add reply to search result list
		c.SearchResults = append(c.SearchResults, msg)

	} else {
		if Context.NoForward {
			return
		}

		if msg.HopLimit > 0 {
			msg.HopLimit--

			// Find next hop
			next, found := c.RoutingTable[msg.Destination]
			if found {
				outMsg := GossipPacket{SearchRep: msg}
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
	if _, found := c.MetafileDatabase[string(metadata.MetaHash)]; !found {
		c.MetafileDatabase[string(metadata.MetaHash)] = &FileDescriptor{metadata.FileName,
		metadata.MetaFile, make([][]string, len(metadata.MetaFile)/32)}
	}

	// Add chunks to lookup database
	numChunks := len(metadata.MetaFile) / 32
	for i := 0; i < numChunks; i++ {
		chunkHash := metadata.MetaFile[i*32 : (i+1)*32]
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
	// Having the metafile is not enough, hence we iterate through the database
	for _, file := range c.SharedFiles {
		if file.FileName == fileName && reflect.DeepEqual(file.MetaHash, fileHash) {
			// Found it!
			return file
		}
	}
	return nil
}

func (c *contextType) FindOrRetrieveFile(fromPeer string, fileName string, metaHash []byte, callback func(*SharedFile, error)) {
	// Check if the file already exists in the local database. If this is the case, return it.
	if file := c.GetFileByNameAndHash(fileName, metaHash); file != nil {
		callback(file, nil)
		return
	}

	if fromPeer == "" {
		// General case: the client has not specified a destination node
		if _, found := c.MetafileDatabase[string(metaHash)]; found {
			if !c.MetafileDatabase[string(metaHash)].HasAllChunks() {
				callback(nil, errors.New("The file has been searched but not all chunks are available"))
				return
			}
		} else {
			callback(nil, errors.New("File not found in download pool (consider searching it first)"))
			return
		}
	}

	// File not found in local database -> download it from the given node
	if fromPeer != c.ThisNodeName {
		_, found := c.RoutingTable[fromPeer]
		if fromPeer != "" && !found {
			callback(nil, errors.New("The given node does not exist in the routing table"))
			return
		}
	} else {
		// Borderline case: the user has requested to download a file from THIS node, but the file does not exist
		callback(nil, errors.New("File not found in local database"))
		return
	}

	// Run async task
	c.DownloadMetaFileFromNode(fromPeer, fileName, metaHash, func(metaFile []byte, err error) {
		if err == nil {
			c.DownloadFileFromNode(fromPeer, fileName, metaHash, metaFile, func(fullFile *SharedFile) {
				if fullFile != nil {
					callback(fullFile, nil)
				} else {
					callback(nil, errors.New(
						"Connection with the destination lost (timed out after 10 retries)"))
				}
			})
		} else {
			callback(nil, err)
		}
	})
}

// DownloadMetaFileFromNode downloads the metafile from a node in an asynchronous way
func (c *contextType) DownloadMetaFileFromNode(fromPeer string, fileName string, metaHash []byte, onCompletion func([]byte, error)) {

	// First, check if the metafile has already been retrieved
	if val, found := c.MetafileDatabase[string(metaHash)]; found {
		onCompletion(val.MetaFile, nil)
		return
	}

	fmt.Printf("DOWNLOADING metafile of %s from %s\n", fileName, fromPeer)
	// Subscribe for replies having the given hash
	metaFile := make(chan []byte)
	c.DownloadSubscriptions[string(metaHash)] = func(data []byte) {
		// Check the metafile for correctness (correct size + hash verification), otherwise drop it
		if VerifyMetafile(metaHash, data) {
			metaFile <- data
		}
	}

	// Run async task to wait for the metafile (or, upon timeout, resend the request)
	go func() {
		metaFileReceived := false
		retryCount := 0
		for !metaFileReceived {
			c.RunSync(func() {
				req := &DataRequest{c.ThisNodeName, fromPeer, 10, fileName, metaHash}
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
					delete(c.DownloadSubscriptions, string(metaHash))

					// Add metafile to local database
					if len(result) > 0 {
						if _, found := c.MetafileDatabase[string(metaHash)]; !found {
							c.MetafileDatabase[string(metaHash)] = &FileDescriptor{fileName,
								result, make([][]string, len(result)/32)}
						}
					}
				})
				// An empty metafile means that the other side does not have the file
				if len(result) > 0 {
					onCompletion(result, nil)
					return
				} else {
					// The node does not have the file (signaled through a nil data field)
					onCompletion(nil, errors.New("The destination node does not have the file"))
					return
				}
			} else if retryCount == 3 {
				c.RunSync(func() {
					// Unsubscribe
					delete(c.DownloadSubscriptions, string(metaHash))
				})

				onCompletion(nil, errors.New("The destination node does not answer (timed out after 3 retries)"))
				return
			}
		}
	}()
}

// If "fromPeer" is not empty, DownloadFileFromNode downloads a file from a SINGLE node, assuming that the metafile has already been retrieved.
// Otherwise, it downloads it from a random number of nodes, assuming that the file has been searched first.
func (c *contextType) DownloadFileFromNode(fromPeer string, fileName string, metaHash []byte, metaFile []byte, onCompletion func(*SharedFile)) {
	go func() {
		numChunks := len(metaFile) / 32
		reconstructedFile := make([]byte, 0)
		for i := 0; i < numChunks; i++ {
			chunkHash := metaFile[i*32 : (i+1)*32]

			chunkData := make(chan []byte)
			c.RunSync(func() {
				c.DownloadSubscriptions[string(chunkHash)] = func(data []byte) {
					// Verify the hash of the chunk. If it not valid, drop the message.
					if VerifyChunk(i, metaFile, data) {
						chunkData <- data
					}
				}
			})

			obtained := false
			// We keep a retry count, so that we stop the request if the destination does not answer within 10 retries
			retryCount := 0
			for !obtained {
				c.RunSync(func() {
					// Request next chunk
					targetPeer := fromPeer
					if targetPeer == "" {
						chunkMap := c.MetafileDatabase[string(metaHash)].ChunkMap[i]
						targetPeer = chunkMap[rand.Intn(len(chunkMap))] // Select random peer
					}

					fmt.Printf("DOWNLOADING %s chunk %d from %s\n", fileName, i+1, targetPeer)
					request := &DataRequest{c.ThisNodeName, targetPeer, 10, "", chunkHash}
					c.ForwardDataRequest(request)
				})
				timeoutTimer := time.After(1 * time.Second) // Retransmit timeout
				select {                                    // Whichever comes first (timeout or metafile)...
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
func (c *contextType) RunSync(event func()) {
	proceed := make(chan bool)
	c.EventQueue <- func() {
		event()
		proceed <- true
	}
	<-proceed
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

func (c *contextType) SearchFiles(keywords []string, budget int, outResults chan string) {
	// Run asynchronously
	go func() {
		expanding := false
		if budget == 0 {
			// Expanding scheme
			budget = 2
			expanding = true
			outResults <- "Automatic budget expansion enabled (from 2 to 32)"
		} else {
			outResults <- "Using a fixed budget of " + fmt.Sprint(budget)
		}

		// MetaHashes seen so far
		seenSet := make(map[string]bool)

		for {

			c.RunSync(func() {
				c.SearchResults = nil // Reset search results
				msg := &SearchRequest{c.ThisNodeName, uint64(budget), keywords}
				c.ForwardSearchRequest(c.ThisNodeAddress, msg)
			})

			// Wait for 1 second until completion
			time.Sleep(time.Second * 1)

			// Collect the results
			proceedCount := 0
			proceed := make(chan bool)
			c.RunSync(func() {
				for _, result := range c.SearchResults {
					for _, match := range result.Results {

						line := fmt.Sprintf("FOUND match %s at %s budget=%d metafile=%s chunks=%s",
							match.FileName, result.Origin, budget, hex.EncodeToString(match.MetafileHash), JoinIntList(match.ChunkMap))
						outResults <- line

						// For each result, check if we have the metafile, otherwise download it
						_, found := c.MetafileDatabase[string(match.MetafileHash)]
						if !found {
							outResults <- fmt.Sprintf("DOWNLOADING metafile of %s from %s", match.FileName, result.Origin)
							proceedCount++
							c.DownloadMetaFileFromNode(result.Origin, match.FileName, match.MetafileHash, func([]byte, error) {
								proceed <- true
							})
						}
					}
				}
			})

			// Wait until all metafiles have been downloaded asynchronously
			for proceedCount > 0 {
				<-proceed
				proceedCount--
			}

			c.RunSync(func() {
				for _, result := range c.SearchResults {
					for _, match := range result.Results {
						if _, found := c.MetafileDatabase[string(match.MetafileHash)]; !found {
							continue
						}

						// Update chunk database
						for _, chunkID := range match.ChunkMap {
							c.MetafileDatabase[string(match.MetafileHash)].AddChunk(int(chunkID - 1), result.Origin)
						}

						seenSet[string(match.MetafileHash)] = true
					}
				}
			})

			resultCount := 0
			c.RunSync(func() {
				// Enumerate the actual whole matches (i.e. matches for which all chunks are available)
				for metaHash := range seenSet {
					if c.MetafileDatabase[metaHash].HasAllChunks() {
						resultCount++
						outResults <- "Downloadable match: " + c.MetafileDatabase[metaHash].FileName + ":" + hex.EncodeToString([]byte(metaHash))
					}
				}
			})

			if resultCount >= 2 {
				// If we have at least 2 results, we can stop without further expanding the ring
				outResults <- "Found 2 results. Stopping."
				break
			}

			// If applicable, double the budget and repeat the request
			if expanding {
				budget *= 2
				outResults <- "Increasing budget to " + fmt.Sprint(budget)
				if budget == 32 {
					outResults <- "Maximum budget of 32 reached. Stopping."
					expanding = false
				}
			} else {
				break
			}
		}

		// Close the channel to signal that the search has ended
		close(outResults)
	}()
}
