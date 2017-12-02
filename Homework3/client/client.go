package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
)

type Message struct {
	Text string
}

func main() {
	uiPort := flag.Int("UIPort", 10001, "the UIPort of the gossiper (default=10001)")
	message := flag.String("msg", "", "the message to send")
	fileName := flag.String("file", "", "the file name to upload")
	fileHash := flag.String("request", "", "the hash of the file to download")
	destination := flag.String("Dest", "", "the destination of a private message (optional)")

	flag.Parse()

	baseUrl := "http://127.0.0.1:" + fmt.Sprint(*uiPort)

	if *fileName != "" && *fileHash != "" && *destination != "" {
		// Download a file to disk
		params := url.Values{}
		params.Set("fileName", *fileName)
		params.Set("fileHash", *fileHash)
		params.Set("filePeer", *destination)
		res, err := http.PostForm(baseUrl+"/download", params)
		if err != nil {
			fmt.Println("Unable to send request")
			return
		}

		switch res.StatusCode {
		case http.StatusOK:
			data, _ := ioutil.ReadAll(res.Body)
			ioutil.WriteFile(*fileName, data, 0644)
		case http.StatusNotFound:
			fmt.Println("File not found")
			response, _ := ioutil.ReadAll(res.Body)
			fmt.Println(string(response))
		default:
			fmt.Println("Request error")
		}
		return

	} else if *fileName != "" {
		// Upload a file from disk
		file, err := os.Open(*fileName)
		if err != nil {
			fmt.Println("Unable to open the file for upload (does it exist?)")
			return
		}
		data, err := ioutil.ReadAll(file)
		if err != nil {
			fmt.Println("Unable to read the file")
			return
		}
		metadata, err := file.Stat()
		if err != nil {
			fmt.Println("Unable to get file metadata")
			return
		}
		file.Close()

		var buffer bytes.Buffer
		writer := multipart.NewWriter(&buffer)
		part, _ := writer.CreateFormFile("uploadedFile", metadata.Name())
		part.Write(data)
		writer.Close()

		request, _ := http.NewRequest("POST", baseUrl+"/upload", &buffer)
		request.Header.Add("Content-Type", writer.FormDataContentType())
		client := &http.Client{}
		response, err := client.Do(request)
		if err != nil {
			fmt.Println("Unable to upload the file (is the gossiper running?)")
		}
		if response.StatusCode == http.StatusBadRequest {
			fmt.Println("Unable to upload the file (400 Bad Request)")
		}

	} else if *destination == "" {
		// Regular gossip message
		data, _ := json.Marshal(*message)
		rs, err := http.Post(baseUrl+"/message", "text/json", bytes.NewBuffer(data))
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
		rs, err := http.Post(baseUrl+"/privateMessage", "text/json", bytes.NewBuffer(data))
		if err != nil || rs.StatusCode != http.StatusOK {
			fmt.Println("Unable to send the private message")
		}

	}
}
