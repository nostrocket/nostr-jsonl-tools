package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Event represents a partial Nostr event with just the fields we need for sorting
type Event struct {
	Pubkey string `json:"pubkey"`
	Raw    []byte
}

// RadixBucket represents a bucket for a specific prefix
type RadixBucket struct {
	Prefix    string
	Events    []Event
	File      *os.File
	EventsLen int
}

const (
	// Maximum events to hold in memory for a bucket before flushing to disk
	maxBucketSize = 100000
	// Number of bits to process in each pass
	bitsPerPass = 4
	// Maximum number of open files to keep
	maxOpenBuckets = 100
	// Maximum event size before writing directly to file
	maxEventSize = 8 * 1024 * 1024 // 8MB
)

func main() {
	// Parse command line arguments
	inputFile := flag.String("file", "", "Input JSONL file to sort")
	outputFile := flag.String("output", "", "Output file path (default: <input>-sorted.jsonl)")
	tempDir := flag.String("temp-dir", "temp-sort", "Directory for temporary files")
	skipCount := flag.Bool("skip-count", false, "Skip counting total events (faster but no percentage progress)")
	workers := flag.Int("workers", runtime.NumCPU(), "Number of worker goroutines")
	debug := flag.Bool("debug", false, "Enable debug output")
	maxPasses := flag.Int("max-passes", 64, "Maximum number of passes to perform (default: 64)")
	flag.Parse()

	// Validate input file
	if *inputFile == "" {
		fmt.Println("Error: Input file is required")
		flag.Usage()
		os.Exit(1)
	}

	// Set default output file if not specified
	if *outputFile == "" {
		*outputFile = *inputFile + "-sorted.jsonl"
	}

	// Get file size for progress reporting
	fileInfo, err := os.Stat(*inputFile)
	if err != nil {
		fmt.Printf("Warning: Could not get file size for progress reporting: %v\n", err)
	} else {
		fileSize := fileInfo.Size()
		fmt.Printf("File size: %.2f GB\n", float64(fileSize)/(1024*1024*1024))
	}

	fmt.Printf("Sorting events from %s by author pubkey using radix sort...\n", *inputFile)
	fmt.Printf("Output will be written to: %s\n", *outputFile)

	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(*tempDir, 0755); err != nil {
		fmt.Printf("Error creating temp directory: %v\n", err)
		os.Exit(1)
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(*outputFile)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Count total valid events for progress reporting
	var totalEvents int
	if !*skipCount {
		var err error
		totalEvents, err = countValidEvents(*inputFile)
		if err != nil {
			fmt.Printf("Error counting events: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Found %d valid events to sort\n", totalEvents)
	}

	// Set up the large events file in the temp directory, not the output directory
	largeEventsFile := filepath.Join(*tempDir, "large_events.jsonl")
	fmt.Printf("Large events will be written to: %s\n", largeEventsFile)

	// Perform the radix sort
	startTime := time.Now()
	processedEvents, err := radixSort(*inputFile, *outputFile, *tempDir, *workers, totalEvents, largeEventsFile, *debug, *maxPasses)
	if err != nil {
		fmt.Printf("Error during sorting: %v\n", err)
		os.Exit(1)
	}

	// Clean up temp directory
	os.RemoveAll(*tempDir)

	elapsed := time.Since(startTime)
	fmt.Printf("\nSorting completed in %v\n", elapsed)
	fmt.Printf("Processed %d events\n", processedEvents)
	fmt.Printf("Output written to: %s\n", *outputFile)

	// Verify the output file exists and has content
	if _, err := os.Stat(*outputFile); err != nil {
		fmt.Printf("ERROR: Output file does not exist or cannot be accessed: %v\n", err)
		
		// Try to copy the last intermediate file to the output file
		if err := copyFile(*inputFile, *outputFile); err != nil {
			fmt.Printf("ERROR: Failed to copy final result to output file: %v\n", err)
		} else {
			fmt.Printf("Successfully copied final result to output file: %s\n", *outputFile)
		}
	} else {
		fmt.Printf("Output file successfully created.\n")
	}
}

// countValidEvents counts the number of valid events in the input file
func countValidEvents(inputFile string) (int, error) {
	fmt.Println("Counting total valid events (this may take a while for large files)...")
	startTime := time.Now()

	// Open the input file
	file, err := os.Open(inputFile)
	if err != nil {
		return 0, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	// Process each line
	var count int
	var lineCount int
	var lastReportTime = time.Now()

	reader := bufio.NewReaderSize(file, 1024*1024) // 1MB buffer
	var line []byte
	var errRead error
	for {
		line, errRead = reader.ReadBytes('\n')
		if errRead != nil && errRead != io.EOF {
			return 0, fmt.Errorf("error reading line: %w", errRead)
		}

		// Process the line even if we hit EOF
		lineCount++

		// Trim whitespace and newlines
		trimmedLine := bytes.TrimSpace(line)

		// Skip empty lines
		if len(trimmedLine) == 0 {
			if errRead == io.EOF {
				break
			}
			continue
		}

		// Extract the pubkey from the JSON
		var partialEvent struct {
			Pubkey string `json:"pubkey"`
		}
		if jsonErr := json.Unmarshal(trimmedLine, &partialEvent); jsonErr != nil {
			// Skip invalid JSON
			continue
		}

		// Skip events with empty pubkeys
		if partialEvent.Pubkey == "" {
			continue
		}

		count++

		// Print progress periodically
		currentTime := time.Now()
		if currentTime.Sub(lastReportTime) > 5*time.Second {
			fmt.Printf("Counted %d valid events so far (processed %d lines)...\n", count, lineCount)
			lastReportTime = currentTime
		}
	}

	if errRead != nil && errRead != io.EOF {
		return 0, fmt.Errorf("error reading file: %w", errRead)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("Finished counting in %v\n", elapsed)
	return count, nil
}

// radixSort performs an external radix sort on the input file
func radixSort(inputFile, outputFile, tempDir string, workers, totalEvents int, largeEventsFile string, debug bool, maxPasses int) (int, error) {
	// Create a file for large events
	largeEvents, err := os.OpenFile(largeEventsFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, fmt.Errorf("error creating large events file: %w", err)
	}
	defer largeEvents.Close()

	// Track the number of large events
	var largeEventCount int

	// Create a mutex for the large events file
	largeEventsMutex := &sync.Mutex{}

	// Calculate the number of hex characters in pubkeys (typically 64 for Nostr)
	pubkeyLength := 64
	// Calculate the number of passes needed
	passes := (pubkeyLength * 4) / bitsPerPass // 4 bits per hex character
	if (pubkeyLength*4)%bitsPerPass != 0 {
		passes++
	}

	// Limit the number of passes if specified
	if maxPasses > 0 && passes > maxPasses {
		passes = maxPasses
	}

	fmt.Printf("Performing radix sort with %d passes (processing %d bits per pass)\n", passes, bitsPerPass)

	// Create a directory for each pass
	var currentPassDir string
	var nextPassDir string

	// First pass reads from the input file
	currentInput := inputFile

	// Process each pass
	var finalEventCount int
	for pass := 0; pass < passes; pass++ {
		passStartTime := time.Now()
		fmt.Printf("Starting pass %d/%d...\n", pass+1, passes)

		// Create directories for this pass
		currentPassDir = filepath.Join(tempDir, fmt.Sprintf("pass-%d", pass))
		if err := os.MkdirAll(currentPassDir, 0755); err != nil {
			return 0, fmt.Errorf("error creating directory for pass %d: %w", pass, err)
		}

		// For the last pass, write directly to the output file
		var nextOutput string
		if pass == passes-1 {
			nextOutput = outputFile
			fmt.Printf("Final pass will write directly to output file: %s\n", outputFile)
		} else {
			// Create directory for the next pass
			nextPassDir = filepath.Join(tempDir, fmt.Sprintf("pass-%d", pass+1))
			if err := os.MkdirAll(nextPassDir, 0755); err != nil {
				return 0, fmt.Errorf("error creating directory for next pass: %w", err)
			}
			nextOutput = filepath.Join(nextPassDir, "input.jsonl")
		}

		// Process this pass
		var passEvents int
		passEvents, err := processPass(currentInput, nextOutput, currentPassDir, pass, pubkeyLength, workers, totalEvents, largeEvents, largeEventsMutex, &largeEventCount, debug)
		if err != nil {
			return 0, fmt.Errorf("error in pass %d: %w", pass, err)
		}

		// Update the processed events count for the final pass
		if pass == passes-1 {
			finalEventCount = passEvents
		}

		// Clean up the previous pass directory if it's not the input file directory
		if pass > 0 {
			prevPassDir := filepath.Join(tempDir, fmt.Sprintf("pass-%d", pass-1))
			os.RemoveAll(prevPassDir)
		}

		// Set up for the next pass
		currentInput = nextOutput

		passElapsed := time.Since(passStartTime)
		fmt.Printf("Completed pass %d/%d in %v\n", pass+1, passes, passElapsed)

		// Check if the output file exists after the final pass
		if pass == passes-1 {
			if _, err := os.Stat(outputFile); err != nil {
				fmt.Printf("WARNING: Output file not found after final pass: %v\n", err)
				
				// Try to copy the last intermediate file to the output file
				if err := copyFile(currentInput, outputFile); err != nil {
					fmt.Printf("ERROR: Failed to copy final result to output file: %v\n", err)
				} else {
					fmt.Printf("Successfully copied final result to output file: %s\n", outputFile)
				}
			} else {
				fmt.Printf("Output file successfully created: %s\n", outputFile)
			}
		}
	}

	// Verify the output file exists and has content
	outputInfo, err := os.Stat(outputFile)
	if err != nil {
		fmt.Printf("Warning: Could not stat output file: %v\n", err)
		
		// If the output file doesn't exist, try to create it from the last intermediate file
		lastPassDir := filepath.Join(tempDir, fmt.Sprintf("pass-%d", passes-1))
		lastInputFile := filepath.Join(lastPassDir, "input.jsonl")
		
		if _, err := os.Stat(lastInputFile); err == nil {
			fmt.Printf("Attempting to copy last intermediate file to output file...\n")
			if err := copyFile(lastInputFile, outputFile); err != nil {
				fmt.Printf("ERROR: Failed to copy last intermediate file to output: %v\n", err)
			} else {
				fmt.Printf("Successfully copied last intermediate file to output file\n")
			}
		}
	} else {
		fmt.Printf("Output file size: %.2f MB\n", float64(outputInfo.Size())/(1024*1024))
	}

	// Count the events in the output file to verify
	events, err := countEventsInFile(outputFile)
	if err != nil {
		fmt.Printf("Warning: Could not count events in output file: %v\n", err)
		return finalEventCount, nil
	}

	fmt.Printf("Final count: %d events in output file\n", events)

	return events, nil
}

// processPass processes a single pass of the radix sort
func processPass(inputFile, outputFile, tempDir string, pass, pubkeyLength, workers, totalEvents int, largeEvents *os.File, largeEventsMutex *sync.Mutex, largeEventCount *int, debug bool) (int, error) {
	// Calculate the bit position for this pass
	// We process from the most significant bits to the least significant
	bitPosition := pubkeyLength*4 - (pass+1)*bitsPerPass
	if bitPosition < 0 {
		bitPosition = 0
	}

	// Create a map to hold buckets
	buckets := make(map[int]*RadixBucket)

	// Open the input file
	file, err := os.Open(inputFile)
	if err != nil {
		return 0, fmt.Errorf("error opening input file: %w", err)
	}
	defer file.Close()

	// Process each line and distribute to buckets
	var processedEvents int
	var lastReportTime = time.Now()

	reader := bufio.NewReaderSize(file, 1024*1024) // 1MB buffer
	var line []byte
	var errRead error
	for {
		line, errRead = reader.ReadBytes('\n')
		if errRead != nil && errRead != io.EOF {
			return 0, fmt.Errorf("error reading line: %w", errRead)
		}

		// Process the line even if we hit EOF
		// Trim whitespace and newlines
		trimmedLine := bytes.TrimSpace(line)

		// Skip empty lines
		if len(trimmedLine) == 0 {
			if errRead == io.EOF {
				break
			}
			continue
		}

		// Extract the pubkey from the JSON
		var partialEvent struct {
			Pubkey string `json:"pubkey"`
		}
		if jsonErr := json.Unmarshal(trimmedLine, &partialEvent); jsonErr != nil {
			// Log invalid JSON but continue processing
			if debug {
				fmt.Printf("Warning: Invalid JSON: %v\n", jsonErr)
			}
			if errRead == io.EOF {
				break
			}
			continue
		}

		// Skip events with empty pubkeys
		if partialEvent.Pubkey == "" {
			if debug {
				fmt.Printf("Warning: Empty pubkey in event\n")
			}
			if errRead == io.EOF {
				break
			}
			continue
		}

		// Calculate the bucket index for this pubkey based on the current bit position
		bucketIndex := getBucketIndex(partialEvent.Pubkey, bitPosition, bitsPerPass)

		// Get or create the bucket
		bucket, exists := buckets[bucketIndex]
		if !exists || bucket == nil {
			// Create a new bucket
			bucketFile := filepath.Join(tempDir, fmt.Sprintf("bucket-%04x.jsonl", bucketIndex))
			file, err := os.OpenFile(bucketFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return 0, fmt.Errorf("error creating bucket file: %w", err)
			}

			bucket = &RadixBucket{
				Prefix:    fmt.Sprintf("%04x", bucketIndex),
				Events:    make([]Event, 0, maxBucketSize),
				File:      file,
				EventsLen: 0,
			}
			buckets[bucketIndex] = bucket
		}

		// Add the event to the bucket
		event := Event{
			Pubkey: partialEvent.Pubkey,
			Raw:    trimmedLine,
		}

		// Check if this is a large event
		if len(event.Raw) > maxEventSize {
			// Write directly to the bucket file
			if err := writeEventToFile(bucket.File, event.Raw); err != nil {
				fmt.Printf("Warning: %v\n", err)
			}

			// Also write to the large events file
			largeEventsMutex.Lock()
			if err := writeEventToFile(largeEvents, event.Raw); err != nil {
				fmt.Printf("Warning: Error writing to large events file: %v\n", err)
			} else {
				(*largeEventCount)++
				if (*largeEventCount)%100 == 0 {
					// Log progress for large events
					fmt.Printf("Found %d large events (>8MB) so far\n", *largeEventCount)
				}
			}
			largeEventsMutex.Unlock()
		} else {
			// Add to in-memory events
			bucket.Events = append(bucket.Events, event)
			bucket.EventsLen++

			// If the bucket is full, flush it to disk
			if bucket.EventsLen >= maxBucketSize {
				// Sort the events in this bucket by pubkey
				sort.Slice(bucket.Events, func(i, j int) bool {
					return bucket.Events[i].Pubkey < bucket.Events[j].Pubkey
				})

				// Write the sorted events to the bucket file
				for _, e := range bucket.Events {
					if err := writeEventToFile(bucket.File, e.Raw); err != nil {
						fmt.Printf("Warning: %v\n", err)
						continue
					}
				}

				// Clear the events array but keep the capacity
				bucket.Events = bucket.Events[:0]
				bucket.EventsLen = 0
			}
		}

		processedEvents++

		// Print progress periodically
		currentTime := time.Now()
		if currentTime.Sub(lastReportTime) > 5*time.Second {
			fmt.Printf("Pass %d: Processed %d events\n", pass+1, processedEvents)
			lastReportTime = currentTime
		}
	}

	if errRead != nil && errRead != io.EOF {
		return 0, fmt.Errorf("error reading file: %w", errRead)
	}

	// Flush all remaining buckets
	for _, bucket := range buckets {
		if bucket.EventsLen > 0 {
			// Sort the events in this bucket by pubkey
			sort.Slice(bucket.Events, func(i, j int) bool {
				return bucket.Events[i].Pubkey < bucket.Events[j].Pubkey
			})

			// Write the sorted events to the bucket file
			for _, event := range bucket.Events {
				if err := writeEventToFile(bucket.File, event.Raw); err != nil {
					fmt.Printf("Warning: %v\n", err)
					continue
				}
			}
		}

		// Close the bucket file
		bucket.File.Close()
	}

	// Now merge all bucket files into the output file
	fmt.Printf("Merging buckets into output file: %s\n", outputFile)

	// Create parent directory for output file if it doesn't exist
	outputDir := filepath.Dir(outputFile)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return 0, fmt.Errorf("error creating output directory: %w", err)
	}

	// Open the output file
	outFile, err := os.Create(outputFile)
	if err != nil {
		return 0, fmt.Errorf("error creating output file: %w", err)
	}
	defer outFile.Close()

	// Process buckets in order (important for radix sort correctness)
	var bucketIndices []int
	for idx := range buckets {
		bucketIndices = append(bucketIndices, idx)
	}

	// Sort bucket indices
	sort.Ints(bucketIndices)

	// Merge each bucket file into the output
	var totalMergedEvents int
	for _, idx := range bucketIndices {
		// Open the bucket file for reading
		bucketFile := filepath.Join(tempDir, fmt.Sprintf("bucket-%04x.jsonl", idx))
		file, err := os.Open(bucketFile)
		if err != nil {
			if os.IsNotExist(err) {
				// Skip if the file doesn't exist (empty bucket)
				continue
			}
			return 0, fmt.Errorf("error opening bucket file for reading: %w", err)
		}

		// Count events in this bucket for debugging
		bucketReader := bufio.NewReaderSize(file, 1024*1024)
		var bucketEvents int
		for {
			_, err := bucketReader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				file.Close()
				return 0, fmt.Errorf("error reading bucket file: %w", err)
			}
			bucketEvents++
		}
		
		// Reset file position for copying
		file.Seek(0, 0)
		
		// Copy the bucket file to the output
		bytesWritten, err := io.Copy(outFile, file)
		file.Close()
		
		if err != nil {
			return 0, fmt.Errorf("error copying bucket to output: %w", err)
		}
		
		if debug {
			fmt.Printf("Merged bucket %04x with %d events, %d bytes\n", idx, bucketEvents, bytesWritten)
		}
		
		totalMergedEvents += bucketEvents
	}
	
	// Flush the output file to ensure all data is written
	if err := outFile.Sync(); err != nil {
		return 0, fmt.Errorf("error flushing output file: %w", err)
	}
	
	fmt.Printf("Merged %d events into output file: %s\n", totalMergedEvents, outputFile)
	
	// Verify the output file was written correctly
	outInfo, err := os.Stat(outputFile)
	if err != nil {
		fmt.Printf("Warning: Could not stat output file after writing: %v\n", err)
	} else {
		fmt.Printf("Output file size after merge: %.2f MB\n", float64(outInfo.Size())/(1024*1024))
	}

	return processedEvents, nil
}

// writeEventToFile writes an event directly to a file without using buffers
func writeEventToFile(file *os.File, event []byte) error {
	// Ensure the event ends with a newline
	if len(event) > 0 && event[len(event)-1] != '\n' {
		if _, err := file.Write(event); err != nil {
			return fmt.Errorf("error writing event: %w", err)
		}
		if _, err := file.Write([]byte("\n")); err != nil {
			return fmt.Errorf("error writing newline: %w", err)
		}
	} else {
		if _, err := file.Write(event); err != nil {
			return fmt.Errorf("error writing event: %w", err)
		}
	}
	return nil
}

// getBucketIndex calculates the bucket index for a pubkey based on the bit position
func getBucketIndex(pubkey string, bitPosition, bitsPerPass int) int {
	// Ensure the pubkey is at least as long as needed
	if len(pubkey) < (bitPosition+bitsPerPass+3)/4 {
		return 0
	}

	// Calculate which hex character(s) we need
	startChar := bitPosition / 4
	endChar := (bitPosition + bitsPerPass - 1) / 4

	// Handle the case where we need bits from multiple characters
	if startChar == endChar {
		// We only need bits from a single character
		charPos := len(pubkey) - 1 - startChar
		if charPos < 0 {
			return 0
		}

		hexChar := pubkey[charPos]
		hexValue, err := strconv.ParseUint(string(hexChar), 16, 8)
		if err != nil {
			return 0
		}

		// Calculate the bit offset within the character
		bitOffset := bitPosition % 4

		// Extract the relevant bits
		return int((hexValue >> bitOffset) & ((1 << bitsPerPass) - 1))
	} else {
		// We need bits from multiple characters
		var result uint64

		// Process each character
		for i := startChar; i <= endChar; i++ {
			charPos := len(pubkey) - 1 - i
			if charPos < 0 {
				continue
			}

			hexChar := pubkey[charPos]
			hexValue, err := strconv.ParseUint(string(hexChar), 16, 8)
			if err != nil {
				continue
			}

			// Calculate how many bits we need from this character
			var bitsFromChar int
			var bitOffset int

			if i == startChar {
				bitOffset = bitPosition % 4
				bitsFromChar = 4 - bitOffset
			} else {
				bitOffset = 0
				bitsFromChar = min(4, bitPosition+bitsPerPass-i*4)
			}

			// Extract the relevant bits and shift them into position
			shiftAmount := (endChar - i) * 4
			if i == endChar {
				shiftAmount = 0
			} else {
				shiftAmount = (i - startChar) * 4
			}

			bits := (hexValue >> bitOffset) & ((1 << bitsFromChar) - 1)
			result |= bits << shiftAmount
		}

		return int(result & ((1 << bitsPerPass) - 1))
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// countEventsInFile counts the number of events in a file
func countEventsInFile(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var count int

	reader := bufio.NewReaderSize(file, 1024*1024) // 1MB buffer
	for {
		_, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
		count++
	}

	return count, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Open source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening source file: %w", err)
	}
	defer sourceFile.Close()

	// Create destination file
	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("error creating destination file: %w", err)
	}
	defer destFile.Close()

	// Copy the contents
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("error copying file contents: %w", err)
	}

	// Ensure all data is written
	if err := destFile.Sync(); err != nil {
		return fmt.Errorf("error flushing destination file: %w", err)
	}

	return nil
}
