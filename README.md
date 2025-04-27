# Nostr Data Tools

This Go project provides a suite of tools for working with Nostr events stored in JSONL files. The tools enable you to extract pubkeys, build follow graphs, filter events, and extract content from specific authors.

## Overview

Nostr (Notes and Other Stuff Transmitted by Relays) is a simple, open protocol that enables global, decentralized, and censorship-resistant social media. This toolkit helps you analyze and work with Nostr event data by providing specialized tools for different tasks:

- **follow-graph**: Build recursive graphs of followers starting from a specified pubkey
- **dgraph-follow**: Store and query Nostr follow graphs in a Dgraph database
- **dgraph-server**: Run a local Dgraph server with data persistence
- **extract-pubkeys**: Extract all unique pubkeys from events in a JSONL file
- **content-extractor**: Extract and store content from specific authors in a SQLite database
- **event-filter**: Filter events by author pubkey and save to a new JSONL file
- **event-splitter**: Split events into separate JSONL files by author pubkey
- **event-radix-sorter**: Sort events using a radix sort algorithm for improved performance
- **event-sorter**: Sort events using external merge sort for handling large files efficiently
- **line-counter**: Count lines in files with support for directories and filtering
- **dedup**: Remove duplicate pubkeys from a list

## Project Structure

```
.
├── cmd/
│   ├── follow-graph/     # Tool for building follow graphs
│   ├── dgraph-follow/    # Tool for storing follow graphs in Dgraph
│   ├── dgraph-server/    # Tool for running a local Dgraph server
│   ├── extract-pubkeys/  # Tool for extracting all pubkeys from events
│   ├── content-extractor/ # Tool for extracting content from specific pubkeys
│   ├── event-filter/     # Tool for filtering events by author pubkey
│   ├── event-splitter/   # Tool for splitting events into files by author
│   ├── event-radix-sorter/ # Tool for sorting events using radix sort algorithm
│   ├── event-sorter/     # Tool for sorting events using external merge sort
│   ├── line-counter/     # Tool for counting lines in files
│   └── dedup/            # Tool for deduplicating pubkey lists
├── pkg/
│   └── nostr/            # Core functionality for Nostr follow graph
├── README.md
└── go.mod
```

## Requirements

- Go 1.18 or higher
- [github.com/nbd-wtf/go-nostr](https://github.com/nbd-wtf/go-nostr) package
- [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) package (for content-extractor)
- [github.com/dgraph-io/dgo/v2](https://github.com/dgraph-io/dgo) package (for dgraph-follow)
- Dgraph binary installed in PATH (for dgraph-server)
- Local Dgraph instance (for dgraph-follow)

## Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/nostr-data-tools.git
cd nostr-data-tools

# Install dependencies
go mod tidy

# Build all tools
go build -o follow-graph ./cmd/follow-graph
go build -o dgraph-follow ./cmd/dgraph-follow
go build -o dgraph-server ./cmd/dgraph-server
go build -o extract-pubkeys ./cmd/extract-pubkeys
go build -o content-extractor ./cmd/content-extractor
go build -o event-filter ./cmd/event-filter
go build -o event-splitter ./cmd/event-splitter
go build -o event-radix-sorter ./cmd/event-radix-sorter
go build -o event-sorter ./cmd/event-sorter
go build -o line-counter ./cmd/line-counter
go build -o dedup ./cmd/dedup
```

## Common Workflows

Here are some common workflows that combine multiple tools:

### Extracting Content from All Events

```bash
# 1. Extract all unique pubkeys from the events
./extract-pubkeys -file events.jsonl -output all-pubkeys.txt

# 2. Extract content from these pubkeys
./content-extractor -file events.jsonl -pubkeys all-pubkeys.txt -db all-content.db
```

### Analyzing a User's Network

```bash
# 1. Build a follow graph starting from a specific user
./follow-graph -file events.jsonl -pubkey <user_npub> -list -output network.txt

# 2. Deduplicate the list of pubkeys
./dedup -input network.txt -output network-dedup.txt

# 3. Extract all events from users in this network
./event-filter -file events.jsonl -pubkeys network-dedup.txt -output network-events.jsonl
```

### Creating a Dataset of Important Users

```bash
# 1. Build a follow graph of users followed by multiple important accounts
./follow-graph -file events.jsonl -pubkey <important_user1> -list -output important1.txt
./follow-graph -file events.jsonl -pubkey <important_user2> -list -output important2.txt

# 2. Combine and deduplicate the lists
cat important1.txt important2.txt > combined.txt
./dedup -input combined.txt -output vips.txt

# 3. Extract events and content from these VIPs
./event-filter -file events.jsonl -pubkeys vips.txt -output vip-events.jsonl
./content-extractor -file events.jsonl -pubkeys vips.txt -db vip-content.db
```

### Organizing Events by Author

```bash
# Split a large JSONL file into separate files by author
./event-splitter -file events.jsonl -output-dir authors

# Process specific authors' events individually
for author in authors/*.jsonl; do
  # Process each author file separately
  echo "Processing $author"
done

# Sort events by author pubkey
./event-radix-sorter -file events.jsonl -output sorted-events.jsonl

# For very large files, use the external merge sort
./event-sorter -file very-large-events.jsonl -output sorted-events.jsonl -memory-limit 500000
```

## Tool Documentation

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

### Dgraph Follow Tool

The dgraph-follow tool stores Nostr follow graphs in a Dgraph database for powerful querying and analysis.

```bash
./dgraph-follow -file <jsonl_file> [-pubkey <root_pubkey>] [-dgraph <address>] [-reset] [-batch-size <size>]
./dgraph-follow -query -pubkey <pubkey> [-depth <depth>] [-limit <limit>] [-dgraph <address>]
./dgraph-follow -stats [-dgraph <address>]
```

Where:
- `<jsonl_file>` is the path to a JSONL file containing Nostr events
- `-pubkey` (optional) is a pubkey to run queries on after import (in hex or npub format)
- `-dgraph` (optional) specifies the Dgraph Alpha address (default: localhost:9080)
- `-reset` (optional) resets the database before importing
- `-batch-size` (optional) specifies the number of mutations to batch together (default: 1000)
- `-query` (optional) runs queries on the database without importing data
- `-depth` (optional) specifies the depth for queries (default: 2)
- `-limit` (optional) specifies the limit for query results (default: 100)
- `-stats` (optional) shows database statistics

The tool:
- Stores the follow graph in a Dgraph database for efficient querying
- Handles large JSONL files with a 10MB buffer
- Supports both hex and npub format pubkeys
- Uses batch mutations for better performance
- Provides several query capabilities:
  - Direct follows of a user
  - Followers of a user
  - Follows of follows (depth 2)
  - Common follows between users (depth 3)
- Shows database statistics including top users by follow/follower count

**Note**: This tool requires a running Dgraph instance. You can use the included `dgraph-server` tool or start one using Docker:
```bash
docker run --rm -it -p 8080:8080 -p 9080:9080 dgraph/standalone:latest
```

### Dgraph Server Tool

The dgraph-server tool runs a local Dgraph instance and persists data to a specified directory.

```bash
./dgraph-server [-data-dir <directory>] [-zero-port <port>] [-alpha-port <port>] [-grpc-port <port>] [-http-port <port>] [-raft-port <port>] [-internal-port <port>] [-verbose]
```

Where:
- `-data-dir` (optional) specifies the directory to store Dgraph data (default: dgraph-data)
- `-zero-port` (optional) specifies the port for Dgraph Zero (default: 5080)
- `-alpha-port` (optional) specifies the port for Dgraph Alpha HTTP (default: 8080)
- `-grpc-port` (optional) specifies the port for Dgraph Alpha gRPC (default: 9080)
- `-http-port` (optional) specifies the port for Dgraph HTTP (default: 8000)
- `-raft-port` (optional) specifies the port for Dgraph Raft (default: 6080)
- `-internal-port` (optional) specifies the port for Dgraph internal communication (default: 7080)
- `-verbose` (optional) enables verbose output

The tool:
- Runs a local Dgraph instance with Zero and Alpha servers
- Persists data to a specified directory
- Configures all necessary ports and directories
- Provides a simple way to start and stop the server
- Handles graceful shutdown on Ctrl+C
- Displays connection information for use with dgraph-follow

**Note**: This tool requires the Dgraph binary to be installed and available in your PATH. You can install it following the instructions at https://dgraph.io/docs/deploy/install/

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

### Content Extractor Tool

The content-extractor tool extracts content from Nostr events for specific pubkeys and stores it in a SQLite database.

```bash
./content-extractor -file <jsonl_file> -pubkeys <pubkeys_file> [-db <database_file>]
```

Where:
- `<jsonl_file>` is the path to a JSONL file containing Nostr events
- `<pubkeys_file>` is a file containing line-separated pubkeys to filter by
- `-db` (optional) specifies the SQLite database file (defaults to "nostr_content.db" if not provided)

The tool:
- Loads a list of pubkeys to filter by from the specified file
- Processes each event in the JSONL file
- If the event's pubkey matches one in the filter list, extracts the content
- Stores the content in a SQLite database, appending new content to existing entries
- Tracks event count and last updated timestamp for each pubkey
- Provides progress statistics during processing

### Event Filter Tool

The event-filter tool extracts complete Nostr events for specific pubkeys and saves them to a new JSONL file.

```bash
./event-filter -file <jsonl_file> -pubkeys <pubkeys_file> [-output <output_file>]
```

Where:
- `<jsonl_file>` is the path to a JSONL file containing Nostr events
- `<pubkeys_file>` is a file containing line-separated pubkeys to filter by
- `-output` (optional) specifies the output JSONL file (defaults to "filtered-events.jsonl" if not provided)

The tool:
- Loads a list of pubkeys to filter by from the specified file
- Processes each event in the JSONL file
- If the event's pubkey matches one in the filter list, copies the entire event to the output file
- Maintains the original JSONL format
- Provides progress statistics during processing

### Event Splitter Tool

The event-splitter tool splits events into separate JSONL files by author pubkey.

```bash
./event-splitter -file <jsonl_file> [-output-dir <output_directory>] [-flush-interval <count>] [-max-open-files <count>]
```

Where:
- `<jsonl_file>` is the path to a JSONL file containing Nostr events
- `-output-dir` (optional) specifies the output directory (defaults to "events-by-author" if not provided)
- `-flush-interval` (optional) specifies how many events to process before flushing writers (defaults to 1000)
- `-max-open-files` (optional) specifies the maximum number of files to keep open at once (defaults to 500)

The tool:
- Processes each event in the JSONL file
- Only includes events of kind 1 (text notes)
- Creates a separate JSONL file for each unique author pubkey in the output directory
- If a file for an author already exists, appends new events to it
- Maintains the original JSONL format for each event
- Uses buffered I/O for better performance
- Periodically flushes data to disk to avoid excessive memory usage
- Intelligently manages file handles to stay within system limits
- Provides progress statistics during processing
- Shows top authors by event count at the end

### Event Radix Sorter Tool

The event-radix-sorter tool sorts Nostr events in a JSONL file by author pubkey using a radix sort algorithm.

```bash
./event-radix-sorter -file <jsonl_file> [-output <output_file>] [-temp-dir <directory>] [-workers <count>] [-max-passes <count>] [-skip-count] [-debug]
```

Where:
- `<jsonl_file>` is the path to a JSONL file containing Nostr events
- `-output` (optional) specifies the output file (defaults to input-sorted.jsonl if not provided)
- `-temp-dir` (optional) specifies the directory to use for temporary files during sorting (default: temp-sort)
- `-workers` (optional) specifies the number of worker goroutines (default: number of CPU cores)
- `-max-passes` (optional) specifies the maximum number of passes to perform (default: 64)
- `-skip-count` (optional) skips counting total events (faster but no percentage progress)
- `-debug` (optional) enables debug output

The tool:
- Sorts events by author pubkey using a radix sort algorithm
- Processes the pubkey bits in multiple passes
- Uses bucketing to efficiently sort very large datasets
- Handles large events (>8MB) separately
- Provides progress statistics during processing
- Cleans up temporary files when done
- Stores all temporary files in the specified temp directory

### Event Sorter Tool

The event-sorter tool sorts Nostr events in a JSONL file by author pubkey using an external merge sort algorithm.

```bash
./event-sorter -file <jsonl_file> [-output <output_file>] [-memory-limit <count>] [-temp-dir <directory>]
```

Where:
- `<jsonl_file>` is the path to a JSONL file containing Nostr events
- `-output` (optional) specifies the output file (defaults to input-sorted.jsonl if not provided)
- `-memory-limit` (optional) specifies the maximum events to hold in memory (default: 1,000,000)
- `-temp-dir` (optional) specifies the directory for temporary files (default: temp-sort)

The tool:
- Uses an external merge sort algorithm to handle large files efficiently
- Splits the input file into sorted chunks that fit in memory
- Merges the sorted chunks into a single output file
- Provides progress reporting during both splitting and merging phases
- Cleans up temporary files when done

### Line Counter Tool

The line-counter tool counts the number of lines in files and directories with various filtering options.

```bash
./line-counter -path <file_or_directory> [-recursive] [-workers <count>] [-sort <none|name|lines|size>] [-include <pattern>] [-exclude <pattern>]
```

Where:
- `<file_or_directory>` is the path to a file or directory to count lines in
- `-recursive` (optional) recursively counts lines in subdirectories
- `-workers` (optional) specifies the number of worker goroutines for parallel processing (default: 4)
- `-sort` (optional) sorts the output by none, name, lines, or size
- `-include` (optional) only includes files matching the specified pattern (e.g., "*.jsonl")
- `-exclude` (optional) excludes files matching the specified pattern (e.g., "*.git*")

The tool:
- Supports counting lines in a single file or all files in a directory
- Includes recursive directory traversal
- Uses parallel processing with configurable worker count
- Provides detailed statistics including line counts and file sizes
- Shows a summary of total lines and size

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

## Tools

### follow-graph

Build recursive graphs of followers starting from a specified pubkey.

```
./follow-graph -file <jsonl_file> -pubkey <root_pubkey> [-depth <depth>] [-output <output_file>]
```

Options:
- `-file`: Path to JSONL file containing Nostr events
- `-pubkey`: Root pubkey to start the graph from
- `-depth`: Maximum depth for recursion (default: 2)
- `-output`: Output file for the graph (default: graph.json)

### dgraph-follow

Store Nostr follow graphs in a local graph database for powerful querying and analysis.

```
./dgraph-follow -file <jsonl_file> [-pubkey <pubkey>] [-dgraph <address>] [-reset] [-batch-size <size>]
./dgraph-follow -query -pubkey <pubkey> [-dgraph <address>]
```

Options:
- `-file`: Path to JSONL file containing Nostr events
- `-pubkey`: Pubkey to query (required for query mode)
- `-dgraph`: Address of the graph server (default: localhost:9080)
- `-reset`: Reset the database before importing
- `-batch-size`: Number of mutations to batch together (default: 100)
- `-query`: Run queries without importing data
- `-depth`: Depth of query (number of hops, default: 1)
- `-limit`: Limit number of results (default: 100)

Requires a running graph server, which can be started using the `dgraph-server` tool.

### dgraph-server

Run a local graph database server that persists data to a specified directory.

```
./dgraph-server [-data-dir <directory>] [-grpc-port <port>] [-reset] [-verbose]
```

Options:
- `-data-dir`: Directory to store graph data (default: graph-data)
- `-grpc-port`: Port for gRPC server (default: 9080)
- `-reset`: Reset the database on startup
- `-verbose`: Enable verbose output

The server uses Badger as the storage backend and provides a simple gRPC API for storing and querying follow relationships.

### extract-pubkeys

Extract all unique pubkeys from events in a JSONL file.

```
./extract-pubkeys -file <jsonl_file> [-output <output_file>]
```

Options:
- `-file`: Path to JSONL file containing Nostr events
- `-output`: Output file for the pubkeys (default: pubkeys.txt)

### content-extractor

Extract and store content from specific authors in a SQLite database.

```
./content-extractor -file <jsonl_file> -pubkeys <pubkeys_file> [-db <database_file>]
```

Options:
- `-file`: Path to JSONL file containing Nostr events
- `-pubkeys`: File containing pubkeys to extract content from
- `-db`: SQLite database file (default: content.db)

### event-filter

Filter events by author pubkey and save to a new JSONL file.

```
./event-filter -file <jsonl_file> -pubkeys <pubkeys_file> [-output <output_file>]
```

Options:
- `-file`: Path to JSONL file containing Nostr events
- `-pubkeys`: File containing pubkeys to filter by
- `-output`: Output file for filtered events (default: filtered.jsonl)

### event-splitter

Split events into separate JSONL files by author pubkey.

```
./event-splitter -file <jsonl_file> [-output-dir <output_directory>]
```

Options:
- `-file`: Path to JSONL file containing Nostr events
- `-output-dir`: Output directory for split files (default: split)

### event-sorter

Sort events using external merge sort for handling large files efficiently.

```
./event-sorter -file <jsonl_file> [-output <output_file>] [-memory-limit <count>] [-temp-dir <directory>]
```

Options:
- `-file`: Path to JSONL file containing Nostr events
- `-output`: Output file for sorted events (default: input-sorted.jsonl)
- `-memory-limit`: Maximum events to hold in memory (default: 1,000,000)
- `-temp-dir`: Directory for temporary files (default: temp-sort)

### Examples

```bash
# Sort events using radix sort algorithm
./event-radix-sorter -file events.jsonl -output radix-sorted.jsonl

# Skip counting events for faster startup with large files
./event-radix-sorter -file large-events.jsonl -skip-count

# Use custom number of worker threads and temp directory
./event-radix-sorter -file events.jsonl -workers 8 -temp-dir /tmp/radix-temp

# For very large files, use the external merge sort
./event-sorter -file very-large-events.jsonl -output sorted-events.jsonl -memory-limit 500000
```

### Performance Considerations

- All tools are designed to handle large JSONL files efficiently
- Each tool uses a 10MB buffer for reading files to handle large events
- Progress reporting is provided for long-running operations
- The event-radix-sorter provides an efficient sorting algorithm for very large datasets
- All tools that create temporary files (event-radix-sorter) ensure they only use disk space in the specified target directory

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
