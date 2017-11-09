package main

import (
	"flag"
	"fmt"
	"os"
	"net/http"
	"encoding/json"
	"bytes"
)

type Message struct {
	Text string
}

func main() {
	uiPort := flag.Int("UIPort", 0, "the UIPort of the gossiper")
	message := flag.String("msg", "", "the message to send")
	destination := flag.String("Dest", "", "the destination of a private message (optional)")

	flag.Parse()

	if *uiPort <= 0 {
		fmt.Println("Error: invalid UI port ")
		os.Exit(1)
	}

	if *destination == "" {
		// Regular gossip message
		data, _ := json.Marshal(*message)
		rs, err := http.Post("http://127.0.0.1:"+fmt.Sprint(*uiPort)+"/message", "text/json", bytes.NewBuffer(data))
		if err != nil || rs.StatusCode != http.StatusOK {
			fmt.Println("Unable to send the gossip message")
		}
	} else {
		// Private message
		type OutgoingMessage struct {
			Destination string
			Content     string
		}

		msg := OutgoingMessage{*destination, *message}
		data, _ := json.Marshal(msg)
		rs, err := http.Post("http://127.0.0.1:"+fmt.Sprint(*uiPort)+"/privateMessage", "text/json", bytes.NewBuffer(data))
		if err != nil || rs.StatusCode != http.StatusOK {
			fmt.Println("Unable to send the private message")
		}

	}
}
