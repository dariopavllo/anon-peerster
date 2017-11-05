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

	flag.Parse()

	if *uiPort <= 0 {
		fmt.Println("Error: invalid UI port ")
		os.Exit(1)
	}

	data, _ := json.Marshal(*message)
	rs, err := http.Post("http://127.0.0.1:" + fmt.Sprint(*uiPort) + "/message", "text/json", bytes.NewBuffer(data))
	if err != nil || rs.StatusCode != http.StatusOK {
		fmt.Println("Unable to send the message")
	}
}
