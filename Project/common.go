package main

import (
	"errors"
	"fmt"
	"net"
)

// FailOnError prints the error and terminates the program, if a non-nil error is given.
func FailOnError(e error) {
	if e != nil {
		panic(e.Error())
	}
}

// AddressToString converts an UDP address structure to a string ipAddress:port
func AddressToString(addr *net.UDPAddr) string {
	return addr.IP.String() + ":" + fmt.Sprint(addr.Port)
}

// CheckAndResolveAddress checks if an ipAddress:port pair is valid and returns it,
// also resolving the domain name (if given).
func CheckAndResolveAddress(address string) (string, error) {
	if len(address) == 0 {
		return "", errors.New("empty address")
	}
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return "", err
	}

	if addr.IP == nil {
		return "", errors.New("invalid IP address")
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
// NumLeadingZeros returns the number of leading zero bits of a hash
func NumLeadingZeros(hash []byte) int {
	count := 0
	for i := 0; i < len(hash); i++ {
		for j := uint(0); j < 8; j++ {
			if hash[i]&(1<<j) == 0 {
				count++
			} else {
				return count
			}
		}
	}
	return count
}

// CompareHashes compares two hashes of the same length.
// It returns -1 if hash1 is lower than hash2, 0 if they are equal, and 1 if hash1 is greater than hash2.
func CompareHashes(hash1 []byte, hash2 []byte) int {
	if len(hash1) != len(hash2) {
		panic("the hash lengths must be the same")
	}

	for i := 0; i < len(hash1); i++ {
		if hash1[i] < hash2[i] {
			return -1
		} else if hash1[i] > hash2[i] {
			return 1
		}
	}
	return 0
}