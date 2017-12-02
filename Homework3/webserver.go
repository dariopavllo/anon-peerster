package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

// InitializeWebServer spawns an HTTP request handler on another thread.
func InitializeWebServer(port int) {
	r := http.NewServeMux()
	r.HandleFunc("/message", handle(handleMessages))
	r.HandleFunc("/node", handle(handleNodes))
	r.HandleFunc("/id", handle(handleId))
	r.HandleFunc("/routes", handle(handleRoutes))
	r.HandleFunc("/privateMessage", handle(handlePrivateMessages))
	r.HandleFunc("/upload", handle(handleFileUpload))
	r.HandleFunc("/download", handleFileDownload)
	r.Handle("/", http.FileServer(http.Dir("webclient")))
	go http.ListenAndServe(":"+fmt.Sprint(port), r)
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
		<-done // Proceed once the request has been handled
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
		for origin, address := range Context.RoutingTable {
			routeList = append(routeList, Route{origin, address})
		}
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

// handlePrivateMessages handles direct messages between nodes.
func handlePrivateMessages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		origin := r.URL.Query().Get("name")
		w.WriteHeader(http.StatusOK)
		messages, found := Context.PrivateMessageLog[origin]
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
			fmt.Printf("PRIVATE SEND \"%s\" TO %s\n", msg.Content, msg.Destination)
			outMsg := Context.BuildPrivateMessage(msg.Destination, msg.Content)
			Context.LogPrivateMessage(Context.ThisNodeAddress, outMsg)
			Context.ForwardPrivateMessage(Context.ThisNodeAddress, outMsg)
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleFileUpload(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		file, header, err := r.FormFile("uploadedFile")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Malformed request"))
			return
		}
		defer file.Close()
		fmt.Printf("FILE UPLOAD %s size %d\n", header.Filename, header.Size)

		var buffer bytes.Buffer
		io.Copy(&buffer, file)

		Context.AddFile(header.Filename, buffer.Bytes())

		// File correctly received
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("File uploaded correctly"))

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("Malformed request"))
	}
}

func handleFileDownload(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		fileName := r.PostFormValue("fileName")
		fileHashStr := r.PostFormValue("fileHash")
		filePeer := r.PostFormValue("filePeer")
		fileHash, err := hex.DecodeString(fileHashStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid file hash"))
			return
		}

		done := make(chan bool)
		var result *SharedFile
		var resultError error
		Context.EventQueue <- func() {
			Context.FindOrRetrieveFile(filePeer, fileName, fileHash, func(res *SharedFile, err error) {
				result = res
				resultError = err
				done <- true
			})
		}
		<-done

		if resultError != nil {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(resultError.Error()))
			return
		}

		http.ServeFile(w, r, "files/"+Context.ThisNodeName+"/"+fileName)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("Malformed request"))
	}
}
