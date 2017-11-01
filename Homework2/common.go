package main

import (
	"fmt"
	"net"
	"os"
)

// FailOnError prints the error and terminates the program, if a non-nil error is given.
func FailOnError(e error) {
	if e != nil {
		fmt.Println("Error: " + e.Error())
		os.Exit(1)
	}
}

// AddressToString converts an UDP address structure to a string ipAddress:port
func AddressToString(addr *net.UDPAddr) string {
	return addr.IP.String() + ":" + fmt.Sprint(addr.Port)
}

// ParseAddress parses an ipAddress:port pair and returns the IP address and the port.
// If a domain name is supplied, it is resolved.
func ParseAddress(address string) (ipAddress string, port int, err error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err == nil {
		ipAddress = addr.IP.String()
		port = addr.Port
	}
	return
}

// CheckAndResolveAddress checks if an ipAddress:port pair is valid and returns it,
// also resolving the domain name (if given).
func CheckAndResolveAddress(address string) (string, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return "", err
	}
	return AddressToString(addr), nil
}

func IsInArray(elem string, arr []string) bool {
	for _, o := range arr {
		if o == elem {
			return true
		}
	}
	return false
}
