package peers

import (
	"encoding/binary"
	"fmt"
	"net"
)

type Peer struct {
	IP   net.IP
	Port uint16
}

func (p Peer) String() string {
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

func Unmarshal(peersBin []byte) ([]Peer, error) {
	const peerSize = 6

	if len(peersBin)%peerSize != 0 {
		return nil, fmt.Errorf("malformed peers, got %d bytes", len(peersBin))
	}

	numPeers := len(peersBin) / peerSize
	peers := make([]Peer, numPeers)

	for i := 0; i < numPeers; i++ {
		offset := i * peerSize
		peers[i].IP = net.IP(peersBin[offset : offset+4])
		peers[i].Port = binary.BigEndian.Uint16(peersBin[offset+4 : offset+6])
	}

	return peers, nil
}
