package main

// RequestListener waits for remote requests (using the given socket)
// and handles them according to the supplied implementation.
type RequestListener struct {
	socket     Socket
	eventQueue chan Event
	Handler    func(data []byte, sender string)
}

// NewRequestListener constructs a new RequestListener, using the given socket.
// All events will be pushed into the main event queue (passed as argument).
func NewRequestListener(socket Socket, eventQueue chan Event) *RequestListener {
	listener := &RequestListener{socket, eventQueue,
		func(data []byte, sender string) {
			// HandleRequest: do nothing
		}}
	return listener
}

// Start activates the listener, which begins listening to the socket.
func (listener *RequestListener) Start() {
	// Run socket receiver in another thread. All events are pushed into the event queue.
	go func() {
		for {
			data, sender := listener.socket.Receive()
			listener.eventQueue <- &Request{listener, data, sender}
		}
	}()
}

func (listener *RequestListener) Close() {
	listener.socket.Close()
}

type Request struct {
	creator *RequestListener
	data    []byte
	sender  string
}

func (request *Request) Handle() {
	request.creator.Handler(request.data, request.sender)
}
