package tracker

import (
	"fmt"
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
	Peers6   string `bencode:"peers6"`
}

func RequestPeers(announce string, infoHash [20]byte, peerID [20]byte, length int) ([]peers.Peer, error) {
	u, err := url.Parse(announce)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "udp":
		return requestPeersUDP(u.Host, infoHash, peerID, length)
	case "http", "https":
		return requestPeersHTTP(announce, infoHash, peerID, length)
	default:
		return nil, fmt.Errorf("unsupported tracker scheme: %s", u.Scheme)
	}
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

	var result []peers.Peer

	if len(raw.Peers) > 0 {
		p, err := peers.Unmarshal([]byte(raw.Peers))
		if err == nil {
			result = append(result, p...)
		}
	}

	if len(raw.Peers6) > 0 {
		p, err := peers.UnmarshalIPv6([]byte(raw.Peers6))
		if err == nil {
			result = append(result, p...)
		}
	}

	return result, nil
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
