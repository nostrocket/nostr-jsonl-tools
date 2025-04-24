package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"nostr-follow-graph/pkg/nostr"
)

func main() {
	// Parse command line flags
	jsonlFilePtr := flag.String("file", "", "JSONL file containing Nostr events")
	rootPubkeyPtr := flag.String("pubkey", "", "Root pubkey to start the graph from")
	outputJSONPtr := flag.Bool("json", false, "Output as JSON")
	useNpubPtr := flag.Bool("npub", false, "Display npub format instead of hex")
	maxDepthPtr := flag.Int("max-depth", 0, "Maximum recursion depth (0 for unlimited)")
	statsPtr := flag.Bool("stats", false, "Show graph statistics")
	statsOnlyPtr := flag.Bool("stats-only", false, "Show only graph statistics without the graph")
	outputFilePtr := flag.String("output", "", "Output file path (if not specified, output to stdout)")
	listPubkeysPtr := flag.Bool("list", false, "Output a line-separated list of pubkeys")

	flag.Parse()

	// Check if we're using positional arguments instead of flags
	args := flag.Args()
	if *jsonlFilePtr == "" && len(args) > 0 {
		*jsonlFilePtr = args[0]
	}
	if *rootPubkeyPtr == "" && len(args) > 1 {
		*rootPubkeyPtr = args[1]
	}

	// Validate required parameters
	if *jsonlFilePtr == "" || *rootPubkeyPtr == "" {
		fmt.Println("Usage: follow-graph -file <jsonl_file> -pubkey <root_pubkey> [-json] [-npub] [-max-depth N] [-stats] [-output file] [-list]")
		fmt.Println("   or: follow-graph <jsonl_file> <root_pubkey>")
		os.Exit(1)
	}

	// Convert npub to hex if needed
	rootPubkey, err := nostr.DecodeNpubIfNeeded(*rootPubkeyPtr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// Build the graph from the JSONL file
	graph, err := nostr.BuildGraphFromJSONL(*jsonlFilePtr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// Determine output writer (file or stdout)
	var outputWriter io.Writer = os.Stdout
	if *outputFilePtr != "" {
		// Automatically add the appropriate file extension if not present
		outputFilePath := *outputFilePtr
		if *listPubkeysPtr && !strings.HasSuffix(outputFilePath, ".txt") {
			// For list format, use .txt extension
			if !strings.Contains(outputFilePath, ".") {
				outputFilePath += ".txt"
			} else {
				// Replace existing extension with .txt
				ext := filepath.Ext(outputFilePath)
				outputFilePath = outputFilePath[:len(outputFilePath)-len(ext)] + ".txt"
			}
		} else if *outputJSONPtr && !strings.HasSuffix(outputFilePath, ".json") {
			// For JSON format, use .json extension
			if !strings.Contains(outputFilePath, ".") {
				outputFilePath += ".json"
			} else {
				// Replace existing extension with .json
				ext := filepath.Ext(outputFilePath)
				outputFilePath = outputFilePath[:len(outputFilePath)-len(ext)] + ".json"
			}
		}

		outputFile, err := os.Create(outputFilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
			os.Exit(1)
		}
		defer outputFile.Close()
		outputWriter = outputFile
		// Print progress to stderr since stdout is redirected to a file
		fmt.Fprintf(os.Stderr, "Output will be written to %s\n", outputFilePath)
	}

	// Output the graph or statistics based on flags
	if *statsOnlyPtr {
		// Only show statistics
		stats := graph.Stats()
		fmt.Fprintln(outputWriter, "\nGraph Statistics:")
		fmt.Fprintf(outputWriter, "Total nodes: %d\n", stats["total_nodes"])

		maxFollowingPubkey, ok1 := stats["max_following_pubkey"].(string)
		maxFollowing, ok2 := stats["max_following"].(int)

		if ok1 && ok2 && maxFollowingPubkey != "" {
			// Try to get the npub if available and requested
			displayKey := maxFollowingPubkey
			if *useNpubPtr {
				if node, exists := graph.Nodes[maxFollowingPubkey]; exists && node.Npub != "" {
					displayKey = node.Npub
				}
			}

			fmt.Fprintf(outputWriter, "Node with most following: %s (%d following)\n",
				displayKey, maxFollowing)
		} else {
			fmt.Fprintln(outputWriter, "No node with following found")
		}
	} else {
		// Show the graph
		if *listPubkeysPtr {
			// Output a line-separated list of pubkeys
			pubkeyList, err := graph.ExportPubkeyList(rootPubkey, *maxDepthPtr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error exporting pubkey list: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprint(outputWriter, string(pubkeyList))
		} else if *outputJSONPtr {
			// Output JSON format
			var jsonData []byte
			var err error

			// Use simple JSON format
			jsonData, err = graph.ExportSimpleJSON(rootPubkey, *maxDepthPtr)

			if err != nil {
				fmt.Fprintf(os.Stderr, "Error exporting JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintln(outputWriter, string(jsonData))
		} else {
			// Print the graph in text format
			fmt.Fprintf(outputWriter, "\nFollow graph starting from %s:\n", rootPubkey)
			graph.PrintGraphToWriter(rootPubkey, 0, *useNpubPtr, *maxDepthPtr, outputWriter)
		}

		// Show statistics after the graph if requested
		if *statsPtr {
			stats := graph.Stats()
			fmt.Fprintln(outputWriter, "\nGraph Statistics:")
			fmt.Fprintf(outputWriter, "Total nodes: %d\n", stats["total_nodes"])

			maxFollowingPubkey, ok1 := stats["max_following_pubkey"].(string)
			maxFollowing, ok2 := stats["max_following"].(int)

			if ok1 && ok2 && maxFollowingPubkey != "" {
				// Try to get the npub if available and requested
				displayKey := maxFollowingPubkey
				if *useNpubPtr {
					if node, exists := graph.Nodes[maxFollowingPubkey]; exists && node.Npub != "" {
						displayKey = node.Npub
					}
				}

				fmt.Fprintf(outputWriter, "Node with most following: %s (%d following)\n",
					displayKey, maxFollowing)
			} else {
				fmt.Fprintln(outputWriter, "No node with following found")
			}
		}
	}
}
