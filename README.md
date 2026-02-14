# torrent

  A fast, lightweight BitTorrent client written in Go — built from scratch with no external
  BitTorrent libraries.

  ## Features

  - **Tracker support** — HTTP and UDP trackers contacted in parallel
  - **Concurrent downloads** — pieces fetched simultaneously from multiple peers via goroutines
  - **Integrity verification** — every piece validated against its SHA-1 hash before writing
  - **Resume support** — interrupted downloads pick up where they left off
  - **Multi-file torrents** — handles both single-file and multi-file `.torrent` files
  - **Speed limiting** — cap download bandwidth in KB/s
  - **IPv4 & IPv6** — full dual-stack peer support

  ## Install

  ```bash
  git clone https://github.com/bossfight33r/torrent
  cd torrent
  go build -o torrent .
  ```

  Requires Go 1.21+.

  ## Usage

  Basic download:

  ```bash
  ./torrent file.torrent
  ```

  Specify output path:

  ```bash
  ./torrent --output ~/Downloads/file.iso file.torrent
  ```

  Limit download speed (KB/s):

  ```bash
  ./torrent --output ~/Downloads/file.iso --limit 5000 file.torrent
  ```

  ## How it works

  1. Parses the `.torrent` file (Bencode format) to extract metadata and piece hashes
  2. Contacts all trackers in parallel (HTTP and UDP) to discover peers
  3. Opens TCP connections to peers and performs the BitTorrent handshake
  4. Downloads pieces concurrently, one goroutine per peer
  5. Verifies each piece against its SHA-1 hash before writing
  6. Streams directly to disk — no full-file buffering in memory

  ## Project structure

  ```
  torrent/
  ├── torrentfile/   — .torrent parsing and download orchestration
  ├── tracker/       — HTTP and UDP tracker communication
  ├── peers/         — peer address parsing (IPv4/IPv6)
  ├── client/        — TCP connection and BitTorrent protocol
  ├── handshake/     — handshake implementation
  ├── message/       — message serialization and parsing
  └── bitfield/      — bitfield for tracking available pieces
