package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: torrent <file.torrent>")
		os.Exit(1)
	}
	fmt.Println("torrent:", os.Args[1])
}
