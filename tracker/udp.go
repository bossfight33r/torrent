package tracker

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"time"
	"torrent/peers"
)

const udpConnectMagic = 0x41727101980

func requestPeersUDP(announceURL string, infoHash [20]byte, peerID [20]byte, length int) ([]peers.Peer, error) {
	conn, err := net.DialTimeout("udp", announceURL, 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(15 * time.Second))

	connID, err := udpConnect(conn)
	if err != nil {
		return nil, err
	}

	return udpAnnounce(conn, connID, infoHash, peerID, length)
}

func udpConnect(conn net.Conn) (uint64, error) {
	txID := rand.Uint32()

	req := make([]byte, 16)
	binary.BigEndian.PutUint64(req[0:8], udpConnectMagic)
	binary.BigEndian.PutUint32(req[8:12], 0)
	binary.BigEndian.PutUint32(req[12:16], txID)

	if _, err := conn.Write(req); err != nil {
		return 0, err
	}

	resp := make([]byte, 16)
	if _, err := conn.Read(resp); err != nil {
		return 0, err
	}

	if binary.BigEndian.Uint32(resp[0:4]) != 0 {
		return 0, fmt.Errorf("expected action 0 in connect response")
	}
	if binary.BigEndian.Uint32(resp[4:8]) != txID {
		return 0, fmt.Errorf("transaction ID mismatch")
	}

	return binary.BigEndian.Uint64(resp[8:16]), nil
}

func udpAnnounce(conn net.Conn, connID uint64, infoHash, peerID [20]byte, length int) ([]peers.Peer, error) {
	txID := rand.Uint32()

	req := make([]byte, 98)
	binary.BigEndian.PutUint64(req[0:8], connID)
	binary.BigEndian.PutUint32(req[8:12], 1)
	binary.BigEndian.PutUint32(req[12:16], txID)
	copy(req[16:36], infoHash[:])
	copy(req[36:56], peerID[:])
	binary.BigEndian.PutUint64(req[56:64], 0)
	binary.BigEndian.PutUint64(req[64:72], uint64(length))
	binary.BigEndian.PutUint64(req[72:80], 0)
	binary.BigEndian.PutUint32(req[80:84], 0)
	binary.BigEndian.PutUint32(req[84:88], 0)
	binary.BigEndian.PutUint32(req[88:92], rand.Uint32())
	binary.BigEndian.PutUint32(req[92:96], ^uint32(0))
	binary.BigEndian.PutUint16(req[96:98], Port)

	if _, err := conn.Write(req); err != nil {
		return nil, err
	}

	resp := make([]byte, 20+6*200)
	n, err := conn.Read(resp)
	if err != nil {
		return nil, err
	}
	if n < 20 {
		return nil, fmt.Errorf("announce response too short: %d bytes", n)
	}
	if binary.BigEndian.Uint32(resp[0:4]) != 1 {
		return nil, fmt.Errorf("expected action 1 in announce response")
	}
	if binary.BigEndian.Uint32(resp[4:8]) != txID {
		return nil, fmt.Errorf("transaction ID mismatch")
	}

	return peers.Unmarshal(resp[20:n])
}
