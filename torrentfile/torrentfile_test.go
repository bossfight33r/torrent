package torrentfile

import (
	"bytes"
	"crypto/sha1"
	"testing"

	"github.com/jackpal/bencode-go"
)

func buildTestTorrent(t *testing.T) []byte {
	t.Helper()

	piece1 := sha1.Sum([]byte("piece 1 data"))
	piece2 := sha1.Sum([]byte("piece 2 data"))
	piece3 := sha1.Sum([]byte("piece 3 data"))
	pieces := string(piece1[:]) + string(piece2[:]) + string(piece3[:])

	raw := bencodeTorrent{
		Announce: "http://tracker.example.com/announce",
		Info: bencodeInfo{
			Name:        "testfile.txt",
			Length:      1024,
			PieceLength: 512,
			Pieces:      pieces,
		},
	}

	var buf bytes.Buffer
	if err := bencode.Marshal(&buf, raw); err != nil {
		t.Fatalf("failed to build test torrent: %v", err)
	}
	return buf.Bytes()
}

func TestParseTorrent(t *testing.T) {
	data := buildTestTorrent(t)

	raw := &bencodeTorrent{}
	if err := bencode.Unmarshal(bytes.NewReader(data), raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	tf, err := raw.toTorrentFile()
	if err != nil {
		t.Fatalf("convert error: %v", err)
	}

	if tf.Announce != "http://tracker.example.com/announce" {
		t.Errorf("Announce: want %q, got %q", "http://tracker.example.com/announce", tf.Announce)
	}
	if tf.Name != "testfile.txt" {
		t.Errorf("Name: want %q, got %q", "testfile.txt", tf.Name)
	}
	if tf.Length != 1024 {
		t.Errorf("Length: want 1024, got %d", tf.Length)
	}
	if tf.PieceLength != 512 {
		t.Errorf("PieceLength: want 512, got %d", tf.PieceLength)
	}
	if len(tf.PieceHashes) != 3 {
		t.Errorf("PieceHashes: want 3, got %d", len(tf.PieceHashes))
	}

	var empty [20]byte
	if tf.InfoHash == empty {
		t.Error("InfoHash is empty")
	}

	t.Logf("OK: Name=%s, Length=%d, Pieces=%d, InfoHash=%x",
		tf.Name, tf.Length, len(tf.PieceHashes), tf.InfoHash)
}

func TestSplitPieceHashes_InvalidLength(t *testing.T) {
	info := bencodeInfo{Pieces: "abcde"}
	_, err := info.splitPieceHashes()
	if err == nil {
		t.Error("expected error for invalid pieces length")
	}
}
