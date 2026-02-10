package torrentfile

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"os"
	"sync/atomic"
	"time"
	"torrent/client"
	"torrent/message"
	"torrent/peers"

	"github.com/charmbracelet/log"
	"github.com/schollz/progressbar/v3"
)

const MaxBlockSize = 16384
const MaxBacklog = 5

type pieceWork struct {
	index  int
	hash   [20]byte
	length int
}

type pieceResult struct {
	index int
	buf   []byte
}

type pieceProgress struct {
	index      int
	client     *client.Client
	buf        []byte
	downloaded int
	requested  int
	backlog    int
}

func (state *pieceProgress) readMessage() error {
	msg, err := state.client.Read()
	if err != nil {
		return err
	}
	if msg == nil {
		return nil
	}
	switch msg.ID {
	case message.MsgUnchoke:
		state.client.Choked = false
	case message.MsgChoke:
		state.client.Choked = true
	case message.MsgHave:
		index, err := message.ParseHave(msg)
		if err != nil {
			return err
		}
		state.client.Bitfield.SetPiece(index)
	case message.MsgPiece:
		n, err := message.ParsePiece(state.index, state.buf, msg)
		if err != nil {
			return err
		}
		state.downloaded += n
		state.backlog--
	}
	return nil
}

func attemptDownloadPiece(c *client.Client, pw *pieceWork) ([]byte, error) {
	state := pieceProgress{
		index:  pw.index,
		client: c,
		buf:    make([]byte, pw.length),
	}

	c.Conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer c.Conn.SetDeadline(time.Time{})

	for state.downloaded < pw.length {
		if !state.client.Choked {
			for state.backlog < MaxBacklog && state.requested < pw.length {
				blockSize := MaxBlockSize
				if pw.length-state.requested < blockSize {
					blockSize = pw.length - state.requested
				}
				if err := c.SendRequest(pw.index, state.requested, blockSize); err != nil {
					return nil, err
				}
				state.backlog++
				state.requested += blockSize
			}
		}
		if err := state.readMessage(); err != nil {
			return nil, err
		}
	}
	return state.buf, nil
}

func checkIntegrity(pw *pieceWork, buf []byte) error {
	hash := sha1.Sum(buf)
	if !bytes.Equal(hash[:], pw.hash[:]) {
		return fmt.Errorf("piece %d failed integrity check", pw.index)
	}
	return nil
}

func (t *TorrentFile) startDownloadWorker(peer peers.Peer, workQueue chan *pieceWork, results chan *pieceResult, active *int32) {
	c, err := client.New(peer, t.PeerID, t.InfoHash)
	if err != nil {
		return
	}
	defer c.Conn.Close()
	atomic.AddInt32(active, 1)
	log.Info("peer connected", "addr", peer.String())
	defer func() {
		atomic.AddInt32(active, -1)
		log.Info("peer disconnected", "addr", peer.String())
	}()

	c.SendUnchoke()
	c.SendInterested()

	for pw := range workQueue {
		if !c.Bitfield.HasPiece(pw.index) {
			workQueue <- pw
			continue
		}

		buf, err := attemptDownloadPiece(c, pw)
		if err != nil {
			workQueue <- pw
			return
		}

		if err := checkIntegrity(pw, buf); err != nil {
			workQueue <- pw
			continue
		}

		c.SendHave(pw.index)
		results <- &pieceResult{pw.index, buf}
	}
}

func (t *TorrentFile) calculatePieceSize(index int) int {
	begin := index * t.PieceLength
	end := begin + t.PieceLength
	if end > t.Length {
		end = t.Length
	}
	return end - begin
}

func (t *TorrentFile) Download(outPath string) error {
	log.Info("starting download", "file", t.Name, "peers", len(t.Peers), "pieces", len(t.PieceHashes))

	workQueue := make(chan *pieceWork, len(t.PieceHashes))
	results := make(chan *pieceResult)
	var active int32

	for index, hash := range t.PieceHashes {
		workQueue <- &pieceWork{index, hash, t.calculatePieceSize(index)}
	}

	for _, peer := range t.Peers {
		go t.startDownloadWorker(peer, workQueue, results, &active)
	}

	name := t.Name
	if len(name) > 30 {
		name = name[:27] + "..."
	}

	bar := progressbar.NewOptions(t.Length,
		progressbar.OptionSetDescription(name),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(200*time.Millisecond),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionUseANSICodes(true),
		progressbar.OptionOnCompletion(func() { fmt.Println() }),
	)

	buf := make([]byte, t.Length)
	for done := 0; done < len(t.PieceHashes); done++ {
		res := <-results
		begin := res.index * t.PieceLength
		copy(buf[begin:], res.buf)
		bar.Add(len(res.buf))
	}
	close(workQueue)

	log.Info("saving", "path", outPath)
	return os.WriteFile(outPath, buf, 0644)
}
