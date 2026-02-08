package tracker

import (
	"net/http"
	"net/url"
	"strconv"
	"time"
	"torrent/peers"

	"github.com/jackpal/bencode-go"
)

const Port uint16 = 6881

type bencodeTrackerResponse struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

func RequestPeers(announce string, infoHash [20]byte, peerID [20]byte, length int) ([]peers.Peer, error) {
	return requestPeersHTTP(announce, infoHash, peerID, length)
}

func requestPeersHTTP(announce string, infoHash [20]byte, peerID [20]byte, length int) ([]peers.Peer, error) {
	trackerURL, err := buildURL(announce, infoHash, peerID, length)
	if err != nil {
		return nil, err
	}

	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(trackerURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw := &bencodeTrackerResponse{}
	if err := bencode.Unmarshal(resp.Body, raw); err != nil {
		return nil, err
	}

	return peers.Unmarshal([]byte(raw.Peers))
}

func buildURL(announce string, infoHash [20]byte, peerID [20]byte, length int) (string, error) {
	base, err := url.Parse(announce)
	if err != nil {
		return "", err
	}

	params := url.Values{
		"info_hash":  []string{string(infoHash[:])},
		"peer_id":    []string{string(peerID[:])},
		"port":       []string{strconv.Itoa(int(Port))},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"compact":    []string{"1"},
		"left":       []string{strconv.Itoa(length)},
	}

	base.RawQuery = params.Encode()
	return base.String(), nil
}
