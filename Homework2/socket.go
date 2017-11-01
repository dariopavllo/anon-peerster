package main

import (
	"fmt"
	"net"
)

// Socket represents a generic socket.
type Socket interface {
	Send(data []byte, address string)
	Receive() ([]byte, string)
	Close()
}

// UdpSocket is an implementation of Socket based on UDP.
type UdpSocket struct {
	connection *net.UDPConn
}

// MakeServerUdpSocket constructs a UDP socket.
// listenAddress can either be a full address "ipAddress:port" or just a port in the form ":port".
// In the latter case, the socket listens on all interfaces.
func MakeServerUdpSocket(listenAddress string) *UdpSocket {
	addr, err := net.ResolveUDPAddr("udp", listenAddress)
	FailOnError(err)

	socket := &UdpSocket{}
	socket.connection, err = net.ListenUDP("udp", addr)
	FailOnError(err)

	return socket
}

func (socket *UdpSocket) Send(data []byte, address string) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err == nil {
		socket.connection.WriteTo(data, addr)
	}
}

func (socket *UdpSocket) Receive() ([]byte, string) {
	buffer := make([]byte, 65536) // Maximum size of UDP datagram: 64 kB
	bytesRead, source, err := socket.connection.ReadFromUDP(buffer)
	FailOnError(err)
	return buffer[:bytesRead], source.IP.String() + ":" + fmt.Sprint(source.Port)
}

func (socket *UdpSocket) Close() {
	socket.connection.Close()
}
