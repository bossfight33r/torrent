package main

import (
	"flag"
	"fmt"
	"os"
	"torrent/torrentfile"
)

func main() {
	output := flag.String("output", "", "output file path")
	limit := flag.Int("limit", 0, "speed limit in KB/s (0 = unlimited)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("usage: torrent [--output <path>] [--limit <KB/s>] <file.torrent>")
		os.Exit(1)
	}

	tf, err := torrentfile.Open(flag.Arg(0))
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	out := *output
	if out == "" {
		out = tf.OutputName()
	}

	if err := tf.FetchPeers(); err != nil {
		fmt.Printf("error fetching peers: %v\n", err)
		os.Exit(1)
	}

	opts := torrentfile.DownloadOptions{
		LimitBytesPerSec: *limit * 1024,
	}

	if err := tf.Download(out, opts); err != nil {
		fmt.Printf("download error: %v\n", err)
		os.Exit(1)
	}
}
