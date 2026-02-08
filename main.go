package main

import (
	"fmt"
	"os"
	"torrent/torrentfile"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: torrent <file.torrent>")
		os.Exit(1)
	}

	tf, err := torrentfile.Open(os.Args[1])
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	if err := tf.FetchPeers(); err != nil {
		fmt.Printf("error fetching peers: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("name:   %s\n", tf.Name)
	fmt.Printf("length: %d bytes\n", tf.Length)
	fmt.Printf("pieces: %d\n", len(tf.PieceHashes))
	fmt.Printf("peers:  %d\n", len(tf.Peers))
}
