package torrentfile

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"torrent/peers"
	"torrent/tracker"

	"github.com/charmbracelet/log"
	"github.com/jackpal/bencode-go"
)

type bencodeFile struct {
	Length int      `bencode:"length"`
	Path   []string `bencode:"path"`
}

type bencodeInfo struct {
	Pieces      string        `bencode:"pieces"`
	PieceLength int           `bencode:"piece length"`
	Length      int           `bencode:"length"`
	Name        string        `bencode:"name"`
	Files       []bencodeFile `bencode:"files"`
}

type bencodeTorrent struct {
	Announce     string      `bencode:"announce"`
	AnnounceList [][]string  `bencode:"announce-list"`
	Info         bencodeInfo `bencode:"info"`
}

type FileEntry struct {
	Path   string
	Length int
	Offset int
}

type TorrentFile struct {
	Announce     string
	AnnounceList [][]string
	InfoHash     [20]byte
	PieceHashes  [][20]byte
	PieceLength  int
	Length       int
	Name         string
	Files        []FileEntry
	Peers        []peers.Peer
	PeerID       [20]byte
}

func (t *TorrentFile) Trackers() []string {
	seen := map[string]bool{}
	var out []string

	add := func(u string) {
		if u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}

	add(t.Announce)
	for _, tier := range t.AnnounceList {
		for _, u := range tier {
			add(u)
		}
	}
	return out
}

func (t *TorrentFile) FetchPeers() error {
	var peerID [20]byte
	if _, err := rand.Read(peerID[:]); err != nil {
		return err
	}
	t.PeerID = peerID

	trackers := t.Trackers()
	log.Info("fetching peers", "trackers", len(trackers))

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, u := range trackers {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			p, err := tracker.RequestPeers(url, t.InfoHash, peerID, t.Length)
			if err != nil {
				log.Debug("tracker failed", "url", url, "err", err)
				return
			}
			mu.Lock()
			t.Peers = append(t.Peers, p...)
			mu.Unlock()
			log.Info("tracker ok", "url", url, "peers", len(p))
		}(u)
	}

	wg.Wait()

	if len(t.Peers) == 0 {
		return fmt.Errorf("no peers found from any tracker")
	}
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

	files, totalLength := b.Info.buildFiles()

	length := b.Info.Length
	if length == 0 {
		length = totalLength
	}

	return TorrentFile{
		Announce:     b.Announce,
		AnnounceList: b.AnnounceList,
		InfoHash:     infoHash,
		PieceHashes:  pieceHashes,
		PieceLength:  b.Info.PieceLength,
		Length:       length,
		Name:         b.Info.Name,
		Files:        files,
	}, nil
}

func (i *bencodeInfo) buildFiles() ([]FileEntry, int) {
	if len(i.Files) == 0 {
		return []FileEntry{{Path: i.Name, Length: i.Length}}, i.Length
	}

	var files []FileEntry
	offset := 0
	for _, f := range i.Files {
		files = append(files, FileEntry{
			Path:   filepath.Join(append([]string{i.Name}, f.Path...)...),
			Length: f.Length,
			Offset: offset,
		})
		offset += f.Length
	}
	return files, offset
}

type bencodeInfoSingle struct {
	Pieces      string `bencode:"pieces"`
	PieceLength int    `bencode:"piece length"`
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
}

type bencodeInfoMulti struct {
	Pieces      string        `bencode:"pieces"`
	PieceLength int           `bencode:"piece length"`
	Name        string        `bencode:"name"`
	Files       []bencodeFile `bencode:"files"`
}

func (i *bencodeInfo) hash() ([20]byte, error) {
	var buf bytes.Buffer
	var err error

	if len(i.Files) > 0 {
		err = bencode.Marshal(&buf, bencodeInfoMulti{
			Pieces:      i.Pieces,
			PieceLength: i.PieceLength,
			Name:        i.Name,
			Files:       i.Files,
		})
	} else {
		err = bencode.Marshal(&buf, bencodeInfoSingle{
			Pieces:      i.Pieces,
			PieceLength: i.PieceLength,
			Length:      i.Length,
			Name:        i.Name,
		})
	}

	if err != nil {
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

func (t *TorrentFile) IsMultiFile() bool {
	return len(t.Files) > 1
}

func (t *TorrentFile) CreateDirs(base string) error {
	for _, f := range t.Files {
		full := filepath.Join(base, f.Path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			return err
		}
	}
	return nil
}

func sanitizeName(name string) string {
	return strings.ReplaceAll(name, "/", "_")
}

func (t *TorrentFile) OutputName() string {
	return sanitizeName(t.Name)
}
