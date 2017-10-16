package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"
)

func main() {

	uiPort := flag.Int("UIPort", 0, "port for the client interface")
	gossipIpPort := flag.String("gossipPort", "", "address/port for the gossiper")
	nodeName := flag.String("name", "", "name of this node")
	peersParams := flag.String("peers", "", "peers separated by commas")

	flag.Parse()

	if *uiPort < 0 {
		FailOnError(errors.New("invalid UI port"))
	}

	if *gossipIpPort == "" {
		FailOnError(errors.New("you must supply a gossip address/port"))
	}

	if *nodeName == "" {
		FailOnError(errors.New("you must specify a name for this node"))
	}

	peerSet := make(map[string]bool)

	// Check if all peer addresses are valid, and resolve them if they contain domain names
	for _, peerAddress := range strings.Split(*peersParams, ",") {
		if peerAddress != "" {
			// Check if the address is valid and resolve its name
			addr, err := CheckAndResolveAddress(peerAddress)
			FailOnError(err)
			peerSet[addr] = true
		}
	}

	// printPeerList prints the peers seen so far
	printPeerList := func() {
		peerList := make([]string, 0)
		for address := range peerSet {
			peerList = append(peerList, address)
		}
		fmt.Println(strings.Join(peerList, ","))
	}

	// Create the event queue as a channel of type Event
	eventQueue := make(chan Event, 10)

	// Define the handler for messages from other peerSet
	gossipSocket := MakeServerUdpSocket(*gossipIpPort)
	peerHandler := NewRequestListener(gossipSocket, eventQueue)
	peerHandler.Handler = func(data []byte, sender string) {
		if sender == *gossipIpPort {
			// This should NEVER happen, unless someone has sent a packet with a spoofed IP address
			// Adding ourselves as a peer would crash the system
			return
		}

		msg := PeerMessage{}
		err := Decode(data, &msg)
		if err != nil {
			// Malformed request -> discard it
			return
		}

		// If the gossiper has not been seen yet: add it to the set
		peerSet[sender] = true

		fmt.Println(msg.Text + " " + msg.OriginalSenderName + " " + msg.RelayPeerAddress)
		printPeerList()

		for peer := range peerSet {
			// Exclude the sender gossiper
			if peer != sender {
				msg.RelayPeerAddress = *gossipIpPort
				gossipSocket.Send(Encode(&msg), peer)
			}
		}
	}
	// Start listening for peer messages in another thread
	peerHandler.Start()

	// If a UI port is given, define the handler for client requests
	if *uiPort != 0 {
		uiSocket := MakeServerUdpSocket(":" + fmt.Sprint(*uiPort))
		clientHandler := NewRequestListener(uiSocket, eventQueue)
		clientHandler.Handler = func(data []byte, sender string) {
			msg := ClientMessage{}
			err := Decode(data, &msg)
			if err != nil {
				// Malformed request -> discard it
				return
			}

			fmt.Println(msg.Text + " N/A N/A")
			printPeerList()

			peerMsg := PeerMessage{}
			peerMsg.Text = msg.Text
			peerMsg.OriginalSenderName = *nodeName // Node name of this gossiper
			for peer := range peerSet {
				peerMsg.RelayPeerAddress = *gossipIpPort // ipAddress:port of this gossiper
				gossipSocket.Send(Encode(&peerMsg), peer)
			}
		}
		// Start listening for client messages in another thread
		clientHandler.Start()
	}

	// Main event loop
	for event := range eventQueue {
		// All events are handled in the main thread
		event.Handle()
	}
}
