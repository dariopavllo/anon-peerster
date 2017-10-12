package main

// Event is the interface that all events in the event queue must implement.
// The method Handle() will be always called in the event loop by the main thread.
type Event interface {
	Handle()
}
