package main

// Peer is the structure associated to a gossiper.
type Peer struct {
	address string
}

func NewPeer(peerAddress string) *Peer {
	return &Peer{peerAddress}
}

func (p *Peer) Address() string {
	return p.address
}

func (p *Peer) String() string {
	return "[" + p.address + "]"
}
