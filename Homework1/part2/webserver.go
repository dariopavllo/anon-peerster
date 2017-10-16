package main

import (
	"net/http"
	"encoding/json"
	"fmt"
	"io/ioutil"
)

// InitializeWebServer spawns an HTTP request handler on another thread.
func InitializeWebServer(port int) {
	r := http.NewServeMux()
	r.HandleFunc("/message", handle(handleMessages))
	r.HandleFunc("/node", handle(handleNodes))
	r.HandleFunc("/id", handle(handleId))
	go http.ListenAndServe(":" + fmt.Sprint(port), r)
}

// handle wraps a handler so that it gets processed on the main event loop.
// Control is returned to the web server thread after the request has been handled.
func handle(callback func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	// All requests are sent to the event queue and handled in the main event loop (i.e. main thread).
	// After the request is processed, the web server thread is unlocked and proceeds
	return func(w http.ResponseWriter, r *http.Request) {
		done := make(chan bool)
		Context.EventQueue <- func() {
			// Enable CORS for all requests
			w.Header().Set("Access-Control-Allow-Origin", "*")
			if r.Method == "OPTIONS" {
				w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "origin, content-type, accept")
				w.WriteHeader(http.StatusOK)
			} else {
				callback(w, r)
			}
			done <- true
		}
		<- done // Proceed once the request has been handled
	}
}

// safeDecode safely decodes an AJAX request in JSON format.
// If an error is detected, a 400 Bad Request status is returned.
func safeDecode(w http.ResponseWriter, r *http.Request, out interface{}) error {
	data, err := ioutil.ReadAll(r.Body)
	if err == nil {
		err := json.Unmarshal(data, out)
		if err == nil {
			return nil
		} else {
			w.WriteHeader(http.StatusBadRequest)
			return err
		}
	} else {
		w.WriteHeader(http.StatusBadRequest)
		return err
	}
}

// handleMessages sends the list of messages to the client, or inserts a new message.
func handleMessages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		w.WriteHeader(http.StatusOK)
		data, _ := json.Marshal(Context.MessageLog)
		w.Write(data)
	case "POST":
		var msg string
		err := safeDecode(w, r, &msg)
		if err == nil {

			w.WriteHeader(http.StatusOK)
			fmt.Printf("CLIENT %s %s\n", msg, Context.ThisNodeName)
			id := Context.AddNewMessage(msg)
			rumorMsg := Context.BuildRumorMessage(Context.ThisNodeName, id)
			randomPeer := Context.RandomPeer([]string{})
			if randomPeer != "" {
				fmt.Printf("MONGERING with %s\n", randomPeer)
				startRumormongering(rumorMsg, randomPeer)
			}
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleNodes sends/updates the list of peers.
func handleNodes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		w.WriteHeader(http.StatusOK)
		peerList := make([]string, 0)
		for peer := range Context.PeerSet {
			peerList = append(peerList, peer)
		}
		data, _ := json.Marshal(peerList)
		w.Write(data)

	case "POST":
		var newPeer string
		err := safeDecode(w, r, &newPeer)
		if err == nil {
			if newPeer == Context.ThisNodeAddress {
				w.WriteHeader(http.StatusBadRequest)
			} else if addr, err := CheckAndResolveAddress(newPeer); err == nil {
				Context.PeerSet[addr] = true
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleNodes sends/changes the name of this node.
func handleId(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		w.WriteHeader(http.StatusOK)
		data, _ := json.Marshal(Context.ThisNodeName)
		w.Write(data)

	case "POST":
		var newName string
		err := safeDecode(w, r, &newName)
		if err == nil {
			Context.ThisNodeName = newName
			w.WriteHeader(http.StatusOK)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
