package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Parse command line flags
	inputFilePtr := flag.String("input", "results.txt", "Input file containing pubkeys (one per line)")
	outputFilePtr := flag.String("output", "", "Output file path (if not specified, will use input-dedup.txt)")
	flag.Parse()

	// Check if input file exists
	if _, err := os.Stat(*inputFilePtr); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Input file %s does not exist\n", *inputFilePtr)
		os.Exit(1)
	}

	// Determine output file name if not specified
	outputFile := *outputFilePtr
	if outputFile == "" {
		ext := filepath.Ext(*inputFilePtr)
		baseName := strings.TrimSuffix(*inputFilePtr, ext)
		outputFile = baseName + "-dedup" + ext
	}

	// Read the input file
	pubkeys, err := readPubkeys(*inputFilePtr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input file: %v\n", err)
		os.Exit(1)
	}

	// Deduplicate the pubkeys
	uniquePubkeys := deduplicate(pubkeys)

	// Write the unique pubkeys to the output file
	err = writePubkeys(outputFile, uniquePubkeys)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully deduplicated %d pubkeys to %d unique pubkeys\n", len(pubkeys), len(uniquePubkeys))
	fmt.Printf("Output written to %s\n", outputFile)
}

// readPubkeys reads pubkeys from a file, one per line
func readPubkeys(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var pubkeys []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			pubkeys = append(pubkeys, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return pubkeys, nil
}

// deduplicate removes duplicate pubkeys while preserving order
func deduplicate(pubkeys []string) []string {
	seen := make(map[string]bool)
	var uniquePubkeys []string

	for _, pubkey := range pubkeys {
		if !seen[pubkey] {
			seen[pubkey] = true
			uniquePubkeys = append(uniquePubkeys, pubkey)
		}
	}

	return uniquePubkeys
}

// writePubkeys writes pubkeys to a file, one per line
func writePubkeys(filePath string, pubkeys []string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, pubkey := range pubkeys {
		_, err := writer.WriteString(pubkey + "\n")
		if err != nil {
			return err
		}
	}

	return writer.Flush()
}
