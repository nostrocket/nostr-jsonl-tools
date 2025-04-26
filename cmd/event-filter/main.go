package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

func main() {
	// Parse command line flags
	inputFile := flag.String("file", "", "Input JSONL file")
	pubkeysFile := flag.String("pubkeys", "", "File containing line-separated pubkeys to filter by")
	outputFile := flag.String("output", "filtered-events.jsonl", "Output JSONL file")
	flag.Parse()

	// Check if required files are provided
	if *inputFile == "" || *pubkeysFile == "" {
		fmt.Println("Error: Input file and pubkeys file are required.")
		fmt.Println("Usage: ./event-filter -file <jsonl_file> -pubkeys <pubkeys_file> [-output <output_file>]")
		os.Exit(1)
	}

	// Load pubkeys from file into a map for quick lookup
	pubkeys, err := loadPubkeys(*pubkeysFile)
	if err != nil {
		fmt.Printf("Error loading pubkeys: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Loaded %d pubkeys to filter by\n", len(pubkeys))

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

	// Create output file
	outFile, err := os.Create(*outputFile)
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer outFile.Close()

	// Create a buffered writer for better performance
	writer := bufio.NewWriter(outFile)
	defer writer.Flush()

	// Create a reader with a large buffer (10MB)
	reader := bufio.NewReaderSize(file, 10*1024*1024)

	// Variables for tracking progress
	lineCount := 0
	matchCount := 0
	startTime := time.Now()
	lastReportTime := startTime
	bytesWritten := 0

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
		trimmedLine := bytes.TrimSpace(line)

		// Skip empty lines
		if len(trimmedLine) == 0 {
			continue
		}

		// Extract the pubkey from the JSON without fully parsing it
		var partialEvent struct {
			Pubkey string `json:"pubkey"`
		}
		if err := json.Unmarshal(trimmedLine, &partialEvent); err != nil {
			// Skip invalid JSON
			continue
		}

		// Check if this pubkey is in our filter list
		if pubkeys[partialEvent.Pubkey] {
			// Write the original line to the output file
			n, err := writer.Write(line)
			if err != nil {
				fmt.Printf("Error writing to output file: %v\n", err)
				continue
			}
			bytesWritten += n
			matchCount++
		}

		// Print progress every 100,000 lines or 5 seconds
		currentTime := time.Now()
		if lineCount%100000 == 0 || currentTime.Sub(lastReportTime) > 5*time.Second {
			elapsed := currentTime.Sub(startTime).Seconds()
			linesPerSec := float64(lineCount) / elapsed
			mbWritten := float64(bytesWritten) / (1024 * 1024)
			fmt.Printf("Processed %d lines (%.2f lines/sec), matched %d events, wrote %.2f MB\n",
				lineCount, linesPerSec, matchCount, mbWritten)
			lastReportTime = currentTime
		}
	}

	// Ensure all data is written
	writer.Flush()

	// Print final statistics
	elapsed := time.Since(startTime).Seconds()
	linesPerSec := float64(lineCount) / elapsed
	mbWritten := float64(bytesWritten) / (1024 * 1024)
	fmt.Printf("\nCompleted processing %d lines in %.2f seconds (%.2f lines/sec)\n",
		lineCount, elapsed, linesPerSec)
	fmt.Printf("Matched and saved %d events (%.2f MB)\n", matchCount, mbWritten)
	fmt.Printf("Output written to: %s\n", *outputFile)
}

// loadPubkeys loads pubkeys from a file into a map
func loadPubkeys(filename string) (map[string]bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	pubkeys := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		pubkey := scanner.Text()
		if pubkey != "" {
			pubkeys[pubkey] = true
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return pubkeys, nil
}
