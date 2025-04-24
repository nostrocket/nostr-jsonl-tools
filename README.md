# Nostr Follow Graph

This Go script parses Nostr events from a JSONL file and builds a recursive graph of followers starting from a specified pubkey.

## Project Structure

```
.
├── cmd/
│   └── follow-graph/     # Main command-line tool
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

# Build the binary
go build -o follow-graph ./cmd/follow-graph
```

## Usage

```bash
# Run without building
go run ./cmd/follow-graph/main.go <jsonl_file> <root_pubkey>

# Or build and run the binary
./follow-graph <jsonl_file> <root_pubkey>
```

Or with named flags:

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
- `-list` (optional) outputs a line-separated list of unique pubkeys in the graph

**Note**: When using the `-output` flag, the script will automatically use the appropriate file extension based on the output format:
- `.json` for JSON output (when using `-json`)
- `.txt` for line-separated list output (when using `-list`)

## Example

```bash
# Basic usage
./follow-graph sample_events.jsonl npub1mygerccwqpzyh9pvp6pv44rskv40zutkfs38t0hqhkvnwlhagp6s3psn5p

# Display pubkeys in npub format
./follow-graph sample_events.jsonl npub1mygerccwqpzyh9pvp6pv44rskv40zutkfs38t0hqhkvnwlhagp6s3psn5p -npub

# Output as JSON
./follow-graph sample_events.jsonl npub1mygerccwqpzyh9pvp6pv44rskv40zutkfs38t0hqhkvnwlhagp6s3psn5p -json

# Limit recursion depth
./follow-graph sample_events.jsonl npub1mygerccwqpzyh9pvp6pv44rskv40zutkfs38t0hqhkvnwlhagp6s3psn5p -max-depth 2

# Show graph statistics
./follow-graph sample_events.jsonl npub1mygerccwqpzyh9pvp6pv44rskv40zutkfs38t0hqhkvnwlhagp6s3psn5p -stats

# Output to a file (will use appropriate extension)
./follow-graph sample_events.jsonl npub1mygerccwqpzyh9pvp6pv44rskv40zutkfs38t0hqhkvnwlhagp6s3psn5p -output results

# Process a large file and output JSON to a file
./follow-graph -file large_events.jsonl -pubkey npub1mygerccwqpzyh9pvp6pv44rskv40zutkfs38t0hqhkvnwlhagp6s3psn5p -json -output results

# Output a line-separated list of pubkeys
./follow-graph sample_events.jsonl npub1mygerccwqpzyh9pvp6pv44rskv40zutkfs38t0hqhkvnwlhagp6s3psn5p -list -output pubkeys
```

## How it Works

1. The script reads a JSONL file containing Nostr events
2. It filters for kind 3 events (contact lists)
3. For each contact list, it extracts the pubkey and its followed pubkeys (p tags)
4. It builds a graph of followers
5. Finally, it prints the recursive graph starting from the specified root pubkey

## Features

- Handles large JSONL files with a 10MB buffer size
- Supports both hex and npub format pubkeys
- Can output the graph in JSON format
- Provides progress updates when processing large files
- Allows limiting the recursion depth for large graphs
- Shows graph statistics (total nodes, node with most following)
- Handles cycles in the graph to prevent infinite recursion
- Can output results to a file instead of stdout
- Can output a line-separated list of unique pubkeys

## Output Format

The output is a tree-like structure showing the follow relationships:
```
rootPubkey (X following)
  followedPubkey1
    followedByPubkey1-1
    followedByPubkey1-2
  followedPubkey2
    ...
```

Note: The script handles cycles in the graph to prevent infinite recursion.
