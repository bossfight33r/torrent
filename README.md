# torrent

A BitTorrent client written in Go

## Features

- HTTP and UDP tracker support
- Parallel piece downloading from multiple peers
- SHA-1 integrity verification for every piece
- Resume interrupted downloads
- Multi-file torrent support
- Download speed display
- Speed limiting
- IPv4 and IPv6 peers

## Usage

```bash
go build -o torrent .
```

```bash
# basic download
./torrent file.torrent

# specify output path
./torrent --output ~/Downloads/file.iso file.torrent

# limit download speed (KB/s)
./torrent --output ~/Downloads/file.iso --limit 5000 file.torrent
```

## How it works

1. Parses the `.torrent` file (Bencode format) to extract metadata and piece hashes
2. Contacts trackers (HTTP/UDP) in parallel to discover peers
3. Performs BitTorrent handshake with each peer over TCP
4. Downloads pieces concurrently from multiple peers using goroutines
5. Verifies each piece against its SHA-1 hash before writing to disk
6. Streams pieces directly to disk instead of buffering in memory

## Project structure

```
torrentfile/   — torrent file parsing and download orchestration
tracker/       — HTTP and UDP tracker communication
peers/         — peer address parsing (IPv4/IPv6)
client/        — TCP connection and BitTorrent protocol
handshake/     — BitTorrent handshake implementation
message/       — message serialization and parsing
bitfield/      — bitfield for tracking piece availability
```
