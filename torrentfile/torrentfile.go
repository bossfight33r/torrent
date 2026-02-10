package torrentfile

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"os"
	"torrent/peers"
	"torrent/tracker"

	"github.com/charmbracelet/log"
	"github.com/jackpal/bencode-go"
)

type bencodeInfo struct {
	Pieces      string `bencode:"pieces"`
	PieceLength int    `bencode:"piece length"`
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
}

type bencodeTorrent struct {
	Announce string      `bencode:"announce"`
	Info     bencodeInfo `bencode:"info"`
}

type TorrentFile struct {
	Announce    string
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
	Peers       []peers.Peer
	PeerID      [20]byte
}

func (t *TorrentFile) FetchPeers() error {
	var peerID [20]byte
	if _, err := rand.Read(peerID[:]); err != nil {
		return err
	}
	t.PeerID = peerID

	log.Info("fetching peers", "tracker", t.Announce)

	p, err := tracker.RequestPeers(t.Announce, t.InfoHash, peerID, t.Length)
	if err != nil {
		return err
	}
	t.Peers = p
	log.Info("found peers", "total", len(t.Peers))
	return nil
}

func Open(path string) (TorrentFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return TorrentFile{}, err
	}
	defer f.Close()

	raw := &bencodeTorrent{}
	if err := bencode.Unmarshal(f, raw); err != nil {
		return TorrentFile{}, err
	}

	return raw.toTorrentFile()
}

func (b *bencodeTorrent) toTorrentFile() (TorrentFile, error) {
	infoHash, err := b.Info.hash()
	if err != nil {
		return TorrentFile{}, err
	}

	pieceHashes, err := b.Info.splitPieceHashes()
	if err != nil {
		return TorrentFile{}, err
	}

	return TorrentFile{
		Announce:    b.Announce,
		InfoHash:    infoHash,
		PieceHashes: pieceHashes,
		PieceLength: b.Info.PieceLength,
		Length:      b.Info.Length,
		Name:        b.Info.Name,
	}, nil
}

func (i *bencodeInfo) hash() ([20]byte, error) {
	var buf bytes.Buffer
	if err := bencode.Marshal(&buf, *i); err != nil {
		return [20]byte{}, err
	}
	return sha1.Sum(buf.Bytes()), nil
}

func (i *bencodeInfo) splitPieceHashes() ([][20]byte, error) {
	buf := []byte(i.Pieces)
	if len(buf)%20 != 0 {
		return nil, fmt.Errorf("malformed pieces, got %d bytes", len(buf))
	}
	count := len(buf) / 20
	hashes := make([][20]byte, count)
	for idx := range hashes {
		copy(hashes[idx][:], buf[idx*20:(idx+1)*20])
	}
	return hashes, nil
}
