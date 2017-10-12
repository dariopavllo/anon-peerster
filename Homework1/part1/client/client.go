package main

import (
	"flag"
	"fmt"
	"github.com/dedis/protobuf"
	"net"
	"os"
)

type Message struct {
	Text string
}

func main() {
	uiPort := flag.Int("UIPort", 0, "the UIPort of the gossiper")
	message := flag.String("msg", "", "the message to send")

	flag.Parse()

	if *uiPort <= 0 {
		fmt.Println("Error: invalid UI port ")
		os.Exit(1)
	}

	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:"+fmt.Sprint(*uiPort))
	conn, _ := net.DialUDP("udp", nil, addr)

	msg := &Message{*message}
	data, _ := protobuf.Encode(msg)

	conn.Write(data)
	conn.Close()
}
