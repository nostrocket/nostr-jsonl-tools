package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
)

// NostrEvent represents a simplified Nostr event structure
type NostrEvent struct {
	Pubkey string `json:"pubkey"`
}

// isValidPubkey checks if a pubkey is valid
func isValidPubkey(pubkey string) bool {
	// Check if the length is correct (64 characters for 32 bytes)
	if len(pubkey) != 64 {
		return false
	}

	// Check if it's a valid hex string
	_, err := hex.DecodeString(pubkey)
	return err == nil
}

func main() {
	// Parse command line flags
	inputFile := flag.String("file", "", "Input JSONL file")
	outputFile := flag.String("output", "output.txt", "Output file name")
	flag.Parse()

	// Check if input file is provided
	if *inputFile == "" {
		fmt.Println("Error: Input file is required. Use -file flag.")
		flag.Usage()
		os.Exit(1)
	}

	// Open the input file
	file, err := os.Open(*inputFile)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Get file info for progress reporting
	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Printf("Error getting file info: %v\n", err)
		os.Exit(1)
	}
	fileSize := fileInfo.Size()
	fmt.Printf("Processing file %s (%d bytes)...\n", *inputFile, fileSize)

	// Use a map to track unique pubkeys
	pubkeys := make(map[string]bool)

	// Variables for tracking progress
	lineCount := 0
	validPubkeyCount := 0
	invalidPubkeyCount := 0

	// Create a reader with a large buffer (10MB)
	reader := bufio.NewReaderSize(file, 10*1024*1024)
	
	// Process each line
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Printf("Error reading line: %v\n", err)
			os.Exit(1)
		}
		
		lineCount++
		
		// Trim whitespace and newlines
		line = bytes.TrimSpace(line)
		
		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		// Parse the JSON
		var event NostrEvent
		if err := json.Unmarshal(line, &event); err != nil {
			// Skip invalid JSON
			continue
		}

		// Check if the pubkey is valid
		if isValidPubkey(event.Pubkey) {
			pubkeys[event.Pubkey] = true
			validPubkeyCount++
		} else if event.Pubkey != "" {
			invalidPubkeyCount++
		}

		// Print progress every 100,000 lines
		if lineCount%100000 == 0 {
			fmt.Printf("Processed %d lines, found %d valid pubkeys.\n", lineCount, len(pubkeys))
		}
	}

	// Convert the map to a sorted slice
	var sortedPubkeys []string
	for pubkey := range pubkeys {
		sortedPubkeys = append(sortedPubkeys, pubkey)
	}
	sort.Strings(sortedPubkeys)

	// Write the sorted pubkeys to the output file
	outFile, err := os.Create(*outputFile)
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer outFile.Close()

	// Write each pubkey on a new line
	for _, pubkey := range sortedPubkeys {
		fmt.Fprintln(outFile, pubkey)
	}

	// Print statistics
	fmt.Printf("Processed %d lines.\n", lineCount)
	fmt.Printf("Found %d valid unique pubkeys.\n", len(pubkeys))
	fmt.Printf("Ignored %d invalid pubkeys.\n", invalidPubkeyCount)
	fmt.Printf("Output written to %s\n", *outputFile)
}
