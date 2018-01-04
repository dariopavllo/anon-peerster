package main

import (
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

func main() {
	uiPort := flag.Int("UIPort", 0, "port for the HTTP/CLI client")
	gossipIpPort := flag.String("gossipAddr", "", "address/port for the gossiper")
	dataDir := flag.String("dataDir", "", "the directory for storing the DB and keys")
	peersParams := flag.String("peers", "", "peers separated by commas")
	powDifficulty := flag.Int("powDifficulty", 16, "proof-of-work difficulty (default: 16 zeros)")

	flag.Parse()

	if *gossipIpPort == "" {
		FailOnError(errors.New("you must supply a gossip address/port (gossipAddr). Use \":PORT\" to listen to all interfaces"))
	}
	Context.ThisNodeAddress = *gossipIpPort

	if *dataDir == "" {
		FailOnError(errors.New("you must specify a database directory (dataDir)"))
	}

	rand.Seed(time.Now().UTC().UnixNano()) // Initialize random seed
	Context.PeerSet = make(map[string]int)
	Context.StatusSubscriptions = make(map[string]func(*StatusPacket))
	Context.PrivateKey, Context.PublicKey = LoadKeyPair(*dataDir)
	Context.DisplayName = Context.PublicKey.DeriveName()
	fmt.Println("The display name of this node is: " + Context.DisplayName)

	Context.Database = NewConnection(*dataDir)
	Context.PowTarget = *powDifficulty
	Context.InsertKeyAnnouncementMessage()

	// Check if all peer addresses are valid, and resolve them if they contain domain names
	for _, peerAddress := range strings.Split(*peersParams, ",") {
		if peerAddress != "" {
			// Check if the address is valid and resolve its name
			addr, err := CheckAndResolveAddress(peerAddress)
			FailOnError(err)
			Context.PeerSet[addr] = Manual
		}
	}

	// printPeerList prints the peers seen so far
	printPeerList := func() {
		peerList := make([]string, 0)
		for address := range Context.PeerSet {
			peerList = append(peerList, address)
		}
		fmt.Println(strings.Join(peerList, ","))
	}

	// Create the event queue as a buffered channel of type Event
	Context.EventQueue = make(chan func(), 10)

	// Define the handler for messages from other peerSet
	Context.GossipSocket = MakeServerUdpSocket(Context.ThisNodeAddress)
	peerHandler := NewRequestListener(Context.GossipSocket)
	peerHandler.Handler = func(data []byte, sender string) {
		if sender == Context.ThisNodeAddress {
			// This should not happen
			return
		}

		msg := &GossipPacket{}
		err := Decode(data, msg)
		if err != nil {
			// Malformed request -> discard it
			return
		}

		// If the gossiper has not been seen yet: add it to the set
		if _, found := Context.PeerSet[sender]; !found {
			Context.PeerSet[sender] = Learned
		}

		if msg.Rumor != nil {
			// Received a rumor message from a peer
			m := msg.Rumor

			fmt.Printf("RUMOR origin %s from %s ID %d contents %s\n", m.Origin, sender, m.ID, string(m.Content))
			printPeerList()

			err := Context.VerifyMessage(m)
			if err != nil {
				// The message failed the verification step
				fmt.Printf("Dropped rumor message due to failed verification (%s)\n", err.Error())
			} else {
				// Valid message
				inserted, _ := Context.TryInsertMessage(m, sender)
				Context.SendStatusMessage(sender) // Send status message in order to acknowledge

				if inserted {
					// This message has not been seen before
					randomPeer := Context.RandomPeer([]string{sender})
					if randomPeer != "" {
						fmt.Printf("MONGERING TEXT with %s\n", randomPeer)
						startRumormongering(m, randomPeer)
					}
				}
			}
		}
		if msg.Status != nil {
			// Received a status message from a peer
			m := msg.Status
			fmt.Printf("STATUS from %s", sender)
			for _, s := range m.Want {
				fmt.Printf(" origin %s nextID %d", s.Identifier, s.NextID)
			}
			fmt.Printf("\n")
			printPeerList()
			if handler, found := Context.StatusSubscriptions[sender]; found {
				// Some task is expecting a status message -> forward it
				handler(m)
			} else {
				// No task is expecting the message -> treat it as an anti-entropy status packet
				synchronizeMessages(m.Want, sender)
			}
		}
	}
	// Start listening for peer messages in another thread
	peerHandler.Start()

	// If a HTTP UI port is given, define the handler for client requests
	if *uiPort != 0 {
		InitializeWebServer(*uiPort)
	}

	// Start anti-entropy routine
	go func() {
		antiEntropyTicker := time.NewTicker(1 * time.Second)
		for _ = range antiEntropyTicker.C {
			Context.EventQueue <- func() {
				// Executed on the main thread
				Context.SendStatusMessage(Context.RandomPeer([]string{}))
			}
		}
	}()

	// Main event loop
	for eventHandler := range Context.EventQueue {
		// All events are handled in the main thread
		eventHandler()
	}
}

// startRumormongering forwards a rumor message to a peer, and the process is optionally repeated
// with another peer according to the coin flip result.
func startRumormongering(msg *RumorMessage, destinationPeerAddress string) {
	if destinationPeerAddress == "" {
		return
	}

	// Forward the rumor message to that peer and wait for a response (or a timeout)
	fwdMessage := GossipPacket{Rumor: msg}

	statusChannel := make(chan *StatusPacket)
	Context.StatusSubscriptions[destinationPeerAddress] = func(statusMessage *StatusPacket) {
		select {
		case statusChannel <- statusMessage:
			{
				// MessageRecord received
			}
		default:
			{
				// Do not block
			}
		}
	}
	Context.GossipSocket.Send(Encode(&fwdMessage), destinationPeerAddress)

	// Run listener in another thread
	go func() {
		timeoutTimer := time.After(1 * time.Second)
		var statusMsg *StatusPacket
		select { // Whichever comes first (timeout or status message)...
		case msg := <-statusChannel:
			statusMsg = msg
		case <-timeoutTimer:
			statusMsg = nil
		}

		Context.EventQueue <- func() {
			// This will be executed on the main thread

			// Unsubscribe from status messages
			delete(Context.StatusSubscriptions, destinationPeerAddress)

			// If a timeout occurs, or the vector clocks match
			if statusMsg == nil || Context.VectorClockEquals(statusMsg.Want) {
				if statusMsg != nil {
					fmt.Printf("IN SYNC WITH %s\n", destinationPeerAddress)
				}

				// Flip a coin
				if rand.Intn(2) == 1 {
					randomPeer := Context.RandomPeer([]string{destinationPeerAddress}) // Avoid selecting this peer again
					if randomPeer != "" {
						fmt.Printf("FLIPPED COIN sending rumor to %s\n", randomPeer)
						startRumormongering(msg, randomPeer)
					}
				}
			} else {
				// The two peers do not agree on the set of messages
				synchronizeMessages(statusMsg.Want, destinationPeerAddress)
			}
		}
	}()
}

// synchronizeMessages compares the vector clocks of this node and the given peer,
// and starts a synchronization job if they differ.
func synchronizeMessages(otherStatus []PeerStatus, destinationPeerAddress string) {
	// If two peers do not agree on the set of messages -> begin exchange
	otherSet, _ := Context.VectorClockDifference(otherStatus)
	inSync := true
	for _, mismatch := range otherSet {
		// The peer has not seen some messages that this node has seen -> send them in order
		id := mismatch.NextID
		inSync = false
		rumor := Context.BuildRumorMessage(mismatch.Identifier, id)
		outMsg := GossipPacket{Rumor: rumor}
		fmt.Printf("MONGERING TEXT with %s\n", destinationPeerAddress)
		Context.GossipSocket.Send(Encode(&outMsg), destinationPeerAddress)
	}

	if inSync {
		fmt.Printf("IN SYNC WITH %s\n", destinationPeerAddress)
	}
}
