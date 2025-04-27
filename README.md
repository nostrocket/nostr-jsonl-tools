# Nostr Data Tools

This Go project provides a suite of tools for working with Nostr events stored in JSONL files. The tools enable you to extract pubkeys, build follow graphs, filter events, and extract content from specific authors.

## Overview

Nostr (Notes and Other Stuff Transmitted by Relays) is a simple, open protocol that enables global, decentralized, and censorship-resistant social media. This toolkit helps you analyze and work with Nostr event data by providing specialized tools for different tasks:

- **follow-graph**: Build recursive graphs of followers starting from a specified pubkey
- **extract-pubkeys**: Extract all unique pubkeys from events in a JSONL file
- **content-extractor**: Extract and store content from specific authors in a SQLite database
- **event-filter**: Filter events by author pubkey and save to a new JSONL file
- **event-splitter**: Split events into separate JSONL files by author pubkey
- **event-radix-sorter**: Sort events using a radix sort algorithm for improved performance
- **line-counter**: Count lines in files with support for directories and filtering
- **dedup**: Remove duplicate pubkeys from a list

## Project Structure

```
.
├── cmd/
│   ├── follow-graph/     # Tool for building follow graphs
│   ├── extract-pubkeys/  # Tool for extracting all pubkeys from events
│   ├── content-extractor/ # Tool for extracting content from specific pubkeys
│   ├── event-filter/     # Tool for filtering events by author pubkey
│   ├── event-splitter/   # Tool for splitting events into files by author
│   ├── event-radix-sorter/ # Tool for sorting events using radix sort algorithm
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

## Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/nostr-data-tools.git
cd nostr-data-tools

# Install dependencies
go mod tidy

# Build all tools
go build -o follow-graph ./cmd/follow-graph
go build -o extract-pubkeys ./cmd/extract-pubkeys
go build -o content-extractor ./cmd/content-extractor
go build -o event-filter ./cmd/event-filter
go build -o event-splitter ./cmd/event-splitter
go build -o event-radix-sorter ./cmd/event-radix-sorter
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
./event-splitter -file <jsonl_file> [-output-dir <output_dir>] [-flush-interval <count>] [-max-open-files <count>]
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

### Examples

```bash
# Sort events using radix sort algorithm
./event-radix-sorter -file events.jsonl -output radix-sorted.jsonl

# Skip counting events for faster startup with large files
./event-radix-sorter -file large-events.jsonl -skip-count

# Use custom number of worker threads and temp directory
./event-radix-sorter -file events.jsonl -workers 8 -temp-dir /tmp/radix-temp
```

### Line Counter Tool

The line-counter tool counts the number of lines in files and directories with various filtering options.

```bash
./line-counter -path <file_or_directory> [-recursive] [-workers <num>] [-sort <none|name|lines|size>] [-include <pattern>] [-exclude <pattern>]
```

Where:
- `<file_or_directory>` is the path to a file or directory to count lines in
- `-recursive` (optional) recursively counts lines in subdirectories
- `-workers` (optional) specifies the number of worker goroutines for parallel processing (default: 4)
- `-sort` (optional) sorts the output by name, lines, or size (default: none)
- `-include` (optional) only includes files matching the specified pattern (e.g., "*.jsonl")
- `-exclude` (optional) excludes files matching the specified pattern (e.g., "*.git*")

The tool:
- Counts lines in a single file or all files in a directory
- Supports recursive directory traversal
- Uses parallel processing for better performance
- Provides filtering options to include/exclude files by pattern
- Displays detailed statistics including line counts and file sizes
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

### Content Extractor Tool

```bash
# Extract content for a list of pubkeys and store in the default database
./content-extractor -file events.jsonl -pubkeys interesting-users.txt

# Extract content and specify a custom database file
./content-extractor -file events.jsonl -pubkeys vips.txt -db vip_content.db
```

### Event Filter Tool

```bash
# Filter events by author pubkey and save to the default output file
./event-filter -file events.jsonl -pubkeys interesting-users.txt

# Filter events and specify a custom output file
./event-filter -file events.jsonl -pubkeys vips.txt -output vip_events.jsonl
```

### Event Splitter Tool

```bash
# Split events into separate files by author pubkey
./event-splitter -file events.jsonl -output-dir split-events
```

### Event Radix Sorter Tool

```bash
# Sort events using radix sort algorithm
./event-radix-sorter -file events.jsonl -output radix-sorted.jsonl

# Skip counting events for faster startup with large files
./event-radix-sorter -file large-events.jsonl -skip-count

# Use custom number of worker threads and temp directory
./event-radix-sorter -file events.jsonl -workers 8 -temp-dir /tmp/radix-temp
```

### Line Counter Tool

```bash
# Count lines in a single file
./line-counter -path events.jsonl

# Count lines in all JSONL files in a directory
./line-counter -path authors/ -include "*.jsonl"

# Count lines recursively and sort by line count (largest first)
./line-counter -path authors/ -recursive -sort lines

# Count lines using 8 worker threads and exclude certain files
./line-counter -path data/ -recursive -workers 8 -exclude "*.tmp"
```

### Deduplication Tool

```bash
# Deduplicate a list of pubkeys
./dedup -input pubkeys.txt -output unique-pubkeys.txt
```

## Performance Considerations

- All tools are designed to handle large JSONL files efficiently
- Each tool uses a 10MB buffer for reading files to handle large events
- Progress reporting is provided for long-running operations
- The event-radix-sorter provides an efficient sorting algorithm for very large datasets
- All tools that create temporary files (event-radix-sorter) ensure they only use disk space in the specified target directory

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
