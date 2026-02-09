package main

import (
	"flag"
	"fmt"
	"os"
	"torrent/torrentfile"
)

func main() {
	output := flag.String("output", "", "output file path")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("usage: torrent [--output <path>] <file.torrent>")
		os.Exit(1)
	}

	tf, err := torrentfile.Open(flag.Arg(0))
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	out := *output
	if out == "" {
		out = tf.Name
	}

	if err := tf.FetchPeers(); err != nil {
		fmt.Printf("error fetching peers: %v\n", err)
		os.Exit(1)
	}

	if err := tf.Download(out); err != nil {
		fmt.Printf("download error: %v\n", err)
		os.Exit(1)
	}
}
