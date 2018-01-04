package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// InitializeWebServer spawns an HTTP request handler on another thread.
func InitializeWebServer(port int) {
	r := http.NewServeMux()
	r.HandleFunc("/message", handleMessages) // Asynchronous (due to proof-of-work)
	r.HandleFunc("/node", handle(handleNodes))
	r.HandleFunc("/id", handle(handleId))
	r.HandleFunc("/routes", handle(handleRoutes))
	r.HandleFunc("/privateMessage", handle(handlePrivateMessages))
	r.Handle("/", http.FileServer(http.Dir("webclient")))
	go http.ListenAndServe("localhost:"+fmt.Sprint(port), r)
}

// handle wraps a handler so that it gets processed on the main event loop.
// Control is returned to the web server thread after the request has been handled.
func handle(callback func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	// All requests are sent to the event queue and handled in the main event loop (i.e. main thread).
	// After the request is processed, the web server thread is unlocked and proceeds
	return func(w http.ResponseWriter, r *http.Request) {
		Context.RunSync(func() {
			// Enable CORS for all requests
			w.Header().Set("Access-Control-Allow-Origin", "*")
			if r.Method == "OPTIONS" {
				w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "origin, content-type, accept")
				w.WriteHeader(http.StatusOK)
			} else {
				callback(w, r)
			}
		})
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

type MessageLogEntry struct {
	FirstSeen   string
	FromNode    string
	SeqID       uint32
	FromAddress string
	Content     string
	Hash        string
}

func ConvertMessageFormat(m *MessageRecord) *MessageLogEntry {
	out := &MessageLogEntry{}
	out.FirstSeen = m.DateSeen
	out.FromAddress = m.FromAddress
	out.FromNode = m.Data.Origin
	out.SeqID = m.Data.ID
	if out.SeqID == 0 {
		// Special message (public key announcement)
		out.Content = "Joined the network for the first time and announced its public key."
	} else if m.Data.Destination == "" {
		// Public message (not encrypted, only signed)
		out.Content = string(m.Data.Content)
	} else {
		// Regular encrypted private message
		text, err := Context.PrivateKey.Decrypt(m.Data.Content)
		if err == nil {
			out.Content = string(text)
		} else {
			// The message is unintelligible
			out.Content = "*** Unable to decrypt the message (sender used wrong key?) ***"
		}
	}
	out.Hash = hex.EncodeToString(m.Data.ComputeHash())
	return out
}

// handleMessages sends the list of messages to the client, or inserts a new message.
func handleMessages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		w.WriteHeader(http.StatusOK)
		var log []*MessageLogEntry
		Context.RunSync(func() {

			messages := Context.Database.GetAllMessagesTo("")
			log = make([]*MessageLogEntry, 0)
			for _, m := range messages {
				log = append(log, ConvertMessageFormat(m))
			}

		})
		data, _ := json.Marshal(log)
		w.Write(data)

	case "POST":
		// The insertion of a new message is asynchronous because of the proof-of-work computation
		var msg string
		err := safeDecode(w, r, &msg)
		if err == nil {

			w.WriteHeader(http.StatusOK)
			fmt.Printf("MESSAGE FROM CLIENT: %s\n", msg)
			id := Context.AddNewMessage(msg) // Blocking on this thread, but not on the main thread
			Context.RunSync(func() {
				rumorMsg := Context.BuildRumorMessage(Context.DisplayName, id)
				randomPeer := Context.RandomPeer([]string{})
				if randomPeer != "" {
					fmt.Printf("MONGERING with %s\n", randomPeer)
					startRumormongering(rumorMsg, randomPeer)
				}
			})
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

		type PeerStruct struct {
			Address string
			Type    int
		}

		peerList := make([]PeerStruct, 0)
		for peer, peerType := range Context.PeerSet {
			peerList = append(peerList, PeerStruct{peer, peerType})
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
				// If the peer is already present, remove it, otherwise add it
				if _, found := Context.PeerSet[addr]; found {
					delete(Context.PeerSet, addr)
				} else {
					Context.PeerSet[addr] = Manual
				}
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleRoutes sends the list of known nodes / routes.
func handleRoutes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		w.WriteHeader(http.StatusOK)

		type Route struct {
			Origin  string
			Address string
		}

		routeList := make([]Route, 0)
		/*for origin, address := range Context.RoutingTable {
			routeList = append(routeList, Route{origin, address})
		}*/
		data, _ := json.Marshal(routeList)
		w.Write(data)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleId sends/changes the name of this node.
func handleId(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		w.WriteHeader(http.StatusOK)
		data, _ := json.Marshal(Context.DisplayName)
		w.Write(data)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handlePrivateMessages handles direct messages between nodes.
func handlePrivateMessages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		//origin := r.URL.Query().Get("name")
		w.WriteHeader(http.StatusOK)
		//messages, found := Context.PrivateMessageLog[origin]
		messages := []MessageLogEntry{}
		found := false
		var data []byte
		if found && len(messages) > 0 {
			data, _ = json.Marshal(messages)
		} else {
			data, _ = json.Marshal([]MessageLogEntry{})
		}
		w.Write(data)

	case "POST":
		type OutgoingMessage struct {
			Destination string
			Content     string
		}

		var msg OutgoingMessage
		err := safeDecode(w, r, &msg)
		if err == nil {
			w.WriteHeader(http.StatusOK)
			//fmt.Printf("PRIVATE SEND \"%s\" TO %s\n", msg.Content, msg.Destination)
			//outMsg := Context.BuildPrivateMessage(msg.Destination, msg.Content)
			//Context.LogPrivateMessage(Context.ThisNodeAddress, outMsg)
			//Context.ForwardPrivateMessage(Context.ThisNodeAddress, outMsg)
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
