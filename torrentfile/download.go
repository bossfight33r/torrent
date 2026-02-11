package torrentfile

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"torrent/client"
	"torrent/message"
	"torrent/peers"

	"github.com/charmbracelet/log"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/time/rate"
)

const MaxBlockSize = 16384
const MaxBacklog = 5

type DownloadOptions struct {
	LimitBytesPerSec int
}

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

func attemptDownloadPiece(c *client.Client, pw *pieceWork, limiter *rate.Limiter) ([]byte, error) {
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
				if limiter != nil {
					limiter.WaitN(context.Background(), blockSize)
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

func (t *TorrentFile) startDownloadWorker(peer peers.Peer, workQueue chan *pieceWork, results chan *pieceResult, active *int32, limiter *rate.Limiter) {
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

		buf, err := attemptDownloadPiece(c, pw, limiter)
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

func (t *TorrentFile) checkExistingPiece(f *os.File, index int) bool {
	pw := &pieceWork{index: index, hash: t.PieceHashes[index], length: t.calculatePieceSize(index)}
	buf := make([]byte, pw.length)
	_, err := f.ReadAt(buf, int64(index*t.PieceLength))
	if err != nil {
		return false
	}
	return checkIntegrity(pw, buf) == nil
}

func (t *TorrentFile) Download(outPath string, opts DownloadOptions) error {
	log.Info("starting download", "file", t.Name, "peers", len(t.Peers), "pieces", len(t.PieceHashes))

	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	var limiter *rate.Limiter
	if opts.LimitBytesPerSec > 0 {
		limiter = rate.NewLimiter(rate.Limit(opts.LimitBytesPerSec), opts.LimitBytesPerSec)
		log.Info("speed limit", "limit", fmt.Sprintf("%d KB/s", opts.LimitBytesPerSec/1024))
	}

	workQueue := make(chan *pieceWork, len(t.PieceHashes))
	results := make(chan *pieceResult)
	var active int32
	skipped := 0

	for index, hash := range t.PieceHashes {
		pw := &pieceWork{index, hash, t.calculatePieceSize(index)}
		if t.checkExistingPiece(f, index) {
			skipped++
			continue
		}
		workQueue <- pw
	}

	if skipped > 0 {
		log.Info("resuming", "skipped", skipped, "remaining", len(t.PieceHashes)-skipped)
	}

	var wg sync.WaitGroup
	for _, peer := range t.Peers {
		wg.Add(1)
		go func(p peers.Peer) {
			defer wg.Done()
			t.startDownloadWorker(p, workQueue, results, &active, limiter)
		}(peer)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	name := t.Name
	if len(name) > 30 {
		name = name[:27] + "..."
	}

	remaining := len(t.PieceHashes) - skipped
	bar := progressbar.NewOptions(t.Length,
		progressbar.OptionSetDescription(name),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(200*time.Millisecond),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionUseANSICodes(true),
		progressbar.OptionOnCompletion(func() { fmt.Println() }),
	)
	bar.Add(skipped * t.PieceLength)

	done := 0
	for res := range results {
		offset := int64(res.index * t.PieceLength)
		if _, err := f.WriteAt(res.buf, offset); err != nil {
			return err
		}
		done++
		bar.Add(len(res.buf))
		if done >= remaining {
			break
		}
	}
	close(workQueue)

	if done < remaining {
		return fmt.Errorf("download incomplete: %d/%d pieces (not enough peers)", done, remaining)
	}

	log.Info("saved", "path", outPath)
	return nil
}
