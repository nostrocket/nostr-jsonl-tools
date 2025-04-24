# Nostr Follow Graph

This Go project provides tools for working with Nostr events, extracting pubkeys, and building follow graphs from JSONL files.

## Project Structure

```
.
├── cmd/
│   ├── follow-graph/     # Tool for building follow graphs
│   ├── extract-pubkeys/  # Tool for extracting all pubkeys from events
│   └── dedup/            # Tool for deduplicating pubkey lists
├── pkg/
│   └── nostr/            # Core functionality for Nostr follow graph
├── README.md
└── go.mod
```

## Requirements

- Go 1.18 or higher
- [github.com/nbd-wtf/go-nostr](https://github.com/nbd-wtf/go-nostr) package

## Installation

```bash
# Install dependencies
go mod tidy

# Build the follow-graph tool
go build -o follow-graph ./cmd/follow-graph

# Build the extract-pubkeys tool
go build -o extract-pubkeys ./cmd/extract-pubkeys

# Build the deduplication tool
go build -o dedup ./cmd/dedup
```

## Usage

### Follow Graph Tool

The follow-graph tool builds a recursive graph of followers starting from a specified pubkey.

```bash
./follow-graph -file <jsonl_file> -pubkey <root_pubkey> [-json] [-npub] [-max-depth <depth>] [-stats] [-output <file>] [-list]
```

Where:
- `<jsonl_file>` is the path to a JSONL file containing Nostr events
- `<root_pubkey>` is the starting pubkey in either hex format or npub format (e.g., `npub1mygerccwqpzyh9pvp6pv44rskv40zutkfs38t0hqhkvnwlhagp6s3psn5p`)
- `-json` (optional) outputs the graph in JSON format
- `-npub` (optional) displays pubkeys in npub format instead of hex
- `-max-depth` (optional) limits the recursion depth for large graphs
- `-stats` (optional) shows graph statistics (total nodes, node with most following)
- `-output` (optional) writes output to the specified file instead of stdout
- `-list` (optional) outputs a line-separated list of pubkeys in the graph

**Note**: When using the `-output` flag, the script will automatically use the appropriate file extension based on the output format:
- `.json` for JSON output (when using `-json`)
- `.txt` for line-separated list output (when using `-list`)

#### Recent Improvements:
- Pubkey validation to ensure only valid 32-byte hex strings are included
- Sorting and deduplication of pubkeys in the output
- Statistics on valid and invalid pubkeys

### Extract Pubkeys Tool

The extract-pubkeys tool extracts all valid pubkeys directly from the "pubkey" field of each Nostr event in a JSONL file.

```bash
./extract-pubkeys -file <jsonl_file> [-output <file>]
```

Where:
- `<jsonl_file>` is the path to a JSONL file containing Nostr events
- `-output` (optional) specifies the output file (defaults to "output.txt" if not provided)

The tool:
- Extracts pubkeys from the "pubkey" field of each event (the author of the event)
- Validates each pubkey to ensure it's a valid 32-byte hex string
- Removes duplicates and sorts the output
- Provides statistics on the number of valid and invalid pubkeys found

### Deduplication Tool

The deduplication tool removes duplicate pubkeys from a line-separated list file:

```bash
# Use default input (results.txt) and output (results-dedup.txt)
./dedup

# Specify a different input file
./dedup -input other-pubkeys.txt

# Specify both input and output files
./dedup -input results.txt -output unique-pubkeys.txt
```

Where:
- `-input` (optional) specifies the input file containing pubkeys (defaults to results.txt)
- `-output` (optional) specifies the output file (defaults to input-dedup.txt if not provided)

## Examples

### Follow Graph Tool

```bash
# Get a list of all pubkeys in the follow graph, starting from a specific user
./follow-graph -file events.jsonl -pubkey npub1mygerccwqpzyh9pvp6pv44rskv40zutkfs38t0hqhkvnwlhagp6s3psn5p -list -output followers

# Generate a JSON representation of the follow graph with a depth limit of 2
./follow-graph -file events.jsonl -pubkey npub1mygerccwqpzyh9pvp6pv44rskv40zutkfs38t0hqhkvnwlhagp6s3psn5p -json -max-depth 2 -output graph
```

### Extract Pubkeys Tool

```bash
# Extract all pubkeys from events and save to the default output.txt
./extract-pubkeys -file events.jsonl

# Extract all pubkeys and save to a custom file
./extract-pubkeys -file events.jsonl -output all-authors.txt
```

### Deduplication Tool

```bash
# Deduplicate a list of pubkeys
./dedup -input pubkeys.txt -output unique-pubkeys.txt
