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
	"sort"
	"sync"
	"time"
)

// FileWriter manages writing to multiple files with buffering
type FileWriter struct {
	baseDir      string
	writers      map[string]*bufio.Writer
	files        map[string]*os.File
	mutex        sync.Mutex
	eventCounts  map[string]int
	closed       map[string]bool // Track which files have been closed
	openCount    int             // Track number of open files
	maxOpenFiles int             // Maximum number of files to keep open
	lastUsed     map[string]time.Time // Track when each file was last used
}

// NewFileWriter creates a new FileWriter
func NewFileWriter(baseDir string, maxOpenFiles int) (*FileWriter, error) {
	// Create the base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", baseDir, err)
	}

	return &FileWriter{
		baseDir:      baseDir,
		writers:      make(map[string]*bufio.Writer),
		files:        make(map[string]*os.File),
		eventCounts:  make(map[string]int),
		closed:       make(map[string]bool),
		maxOpenFiles: maxOpenFiles,
		lastUsed:     make(map[string]time.Time),
	}, nil
}

// WriteEvent writes an event to the appropriate file based on pubkey
func (fw *FileWriter) WriteEvent(pubkey string, data []byte) error {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	// Get or create writer for this pubkey
	writer, err := fw.getWriter(pubkey)
	if err != nil {
		return err
	}

	// Update last used time for this pubkey
	fw.lastUsed[pubkey] = time.Now()

	// Write the data with a newline
	if _, err := writer.Write(data); err != nil {
		return err
	}
	if _, err := writer.Write([]byte("\n")); err != nil {
		return err
	}

	// Increment event count
	fw.eventCounts[pubkey]++

	return nil
}

// getWriter returns a buffered writer for the given pubkey
func (fw *FileWriter) getWriter(pubkey string) (*bufio.Writer, error) {
	// Check if we need to close some files to stay under the limit
	if fw.openCount >= fw.maxOpenFiles && fw.maxOpenFiles > 0 {
		fw.closeOldestFiles(fw.maxOpenFiles / 5) // Close 20% of the files
	}

	// Check if this file has been closed previously
	if fw.closed[pubkey] {
		// Reopen the file
		filename := filepath.Join(fw.baseDir, pubkey+".jsonl")
		file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to reopen file %s: %w", filename, err)
		}
		
		// Create a new buffered writer
		writer := bufio.NewWriter(file)
		fw.writers[pubkey] = writer
		fw.files[pubkey] = file
		fw.closed[pubkey] = false
		fw.openCount++
		
		return writer, nil
	}

	// Check if we already have a writer for this pubkey
	if writer, ok := fw.writers[pubkey]; ok {
		return writer, nil
	}

	// Create a new file for this pubkey
	filename := filepath.Join(fw.baseDir, pubkey+".jsonl")
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filename, err)
	}

	// Create a buffered writer
	writer := bufio.NewWriter(file)
	fw.writers[pubkey] = writer
	fw.files[pubkey] = file
	fw.closed[pubkey] = false
	fw.openCount++
	
	// Initialize event count if this is a new pubkey
	if _, exists := fw.eventCounts[pubkey]; !exists {
		fw.eventCounts[pubkey] = 0
	}
	
	return writer, nil
}

// closeOldestFiles closes the least recently used files
func (fw *FileWriter) closeOldestFiles(count int) error {
	// Create a slice of pubkeys sorted by last used time
	type pubkeyTime struct {
		pubkey string
		time   time.Time
	}
	
	pubkeys := make([]pubkeyTime, 0, len(fw.lastUsed))
	for pubkey, lastUsed := range fw.lastUsed {
		// Only include pubkeys that have open files
		if _, ok := fw.files[pubkey]; ok && !fw.closed[pubkey] {
			pubkeys = append(pubkeys, pubkeyTime{pubkey, lastUsed})
		}
	}
	
	// Sort by last used time (oldest first)
	sort.Slice(pubkeys, func(i, j int) bool {
		return pubkeys[i].time.Before(pubkeys[j].time)
	})
	
	// Close the oldest files
	closedCount := 0
	for _, pt := range pubkeys {
		if closedCount >= count {
			break
		}
		
		pubkey := pt.pubkey
		writer := fw.writers[pubkey]
		file := fw.files[pubkey]
		
		// Flush and close
		if err := writer.Flush(); err != nil {
			fmt.Printf("Error flushing writer for %s: %v\n", pubkey, err)
		}
		
		if err := file.Close(); err != nil {
			fmt.Printf("Error closing file for %s: %v\n", pubkey, err)
		}
		
		// Mark as closed
		fw.closed[pubkey] = true
		delete(fw.writers, pubkey)
		delete(fw.files, pubkey)
		fw.openCount--
		closedCount++
	}
	
	return nil
}

// Flush flushes all writers and closes all files
func (fw *FileWriter) Flush() error {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	var lastErr error
	for pubkey, writer := range fw.writers {
		if err := writer.Flush(); err != nil {
			lastErr = err
			fmt.Printf("Error flushing writer for %s: %v\n", pubkey, err)
		}
	}

	for pubkey, file := range fw.files {
		if err := file.Close(); err != nil {
			lastErr = err
			fmt.Printf("Error closing file for %s: %v\n", pubkey, err)
		}
		fw.closed[pubkey] = true
	}
	
	// Clear the maps
	fw.writers = make(map[string]*bufio.Writer)
	fw.files = make(map[string]*os.File)
	fw.openCount = 0

	return lastErr
}

// GetStats returns statistics about the written files
func (fw *FileWriter) GetStats() map[string]int {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	// Make a copy to avoid race conditions
	stats := make(map[string]int, len(fw.eventCounts))
	for k, v := range fw.eventCounts {
		stats[k] = v
	}
	return stats
}

func main() {
	// Parse command line flags
	inputFile := flag.String("file", "", "Input JSONL file containing Nostr events")
	outputDir := flag.String("output-dir", "events-by-author", "Output directory for per-author JSONL files")
	flushInterval := flag.Int("flush-interval", 1000, "Number of events to process before flushing writers")
	maxOpenFiles := flag.Int("max-open-files", 500, "Maximum number of files to keep open at once")
	flag.Parse()

	// Check if input file is provided
	if *inputFile == "" {
		fmt.Println("Error: Input file is required.")
		fmt.Println("Usage: ./event-splitter -file <jsonl_file> [-output-dir <directory>] [-flush-interval <count>] [-max-open-files <count>]")
		os.Exit(1)
	}

	// Create the file writer
	fileWriter, err := NewFileWriter(*outputDir, *maxOpenFiles)
	if err != nil {
		fmt.Printf("Error creating file writer: %v\n", err)
		os.Exit(1)
	}
	defer fileWriter.Flush()

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

	// Create a reader with a large buffer (10MB)
	reader := bufio.NewReaderSize(file, 10*1024*1024)

	// Variables for tracking progress
	lineCount := 0
	eventCount := 0
	startTime := time.Now()
	lastReportTime := startTime
	uniqueAuthors := make(map[string]bool)
	lastFlushCount := 0

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

		// Extract the pubkey and kind from the JSON without fully parsing it
		var partialEvent struct {
			Pubkey string `json:"pubkey"`
			Kind   int    `json:"kind"`
		}
		if err := json.Unmarshal(trimmedLine, &partialEvent); err != nil {
			// Skip invalid JSON
			fmt.Printf("Warning: Invalid JSON at line %d: %v\n", lineCount, err)
			continue
		}

		// Skip events with empty pubkeys
		if partialEvent.Pubkey == "" {
			fmt.Printf("Warning: Empty pubkey at line %d\n", lineCount)
			continue
		}
		
		// Only process events of kind 1 (text notes)
		if partialEvent.Kind != 1 {
			continue
		}
		
		// Track unique authors
		uniqueAuthors[partialEvent.Pubkey] = true

		// Write the event to the appropriate file
		if err := fileWriter.WriteEvent(partialEvent.Pubkey, trimmedLine); err != nil {
			fmt.Printf("Error writing event: %v\n", err)
			continue
		}

		eventCount++
		
		// Check if we need to flush based on event count
		if eventCount - lastFlushCount >= *flushInterval {
			if err := fileWriter.Flush(); err != nil {
				fmt.Printf("Error flushing writers: %v\n", err)
			}
			lastFlushCount = eventCount
			
			// Print progress
			currentTime := time.Now()
			elapsed := currentTime.Sub(startTime).Seconds()
			linesPerSec := float64(lineCount) / elapsed
			fmt.Printf("Processed %d lines (%.2f lines/sec), %d events, %d unique authors\n",
				lineCount, linesPerSec, eventCount, len(uniqueAuthors))
		}
		
		// Print progress every 100,000 lines or 5 seconds
		currentTime := time.Now()
		if lineCount%100000 == 0 || currentTime.Sub(lastReportTime) > 5*time.Second {
			elapsed := currentTime.Sub(startTime).Seconds()
			linesPerSec := float64(lineCount) / elapsed
			fmt.Printf("Processed %d lines (%.2f lines/sec), %d events, %d unique authors\n",
				lineCount, linesPerSec, eventCount, len(uniqueAuthors))
			lastReportTime = currentTime
		}
	}

	// Final flush of all writers
	if err := fileWriter.Flush(); err != nil {
		fmt.Printf("Error during final flush: %v\n", err)
	}

	// Print final statistics
	elapsed := time.Since(startTime).Seconds()
	linesPerSec := float64(lineCount) / elapsed
	fmt.Printf("\nCompleted processing %d lines in %.2f seconds (%.2f lines/sec)\n",
		lineCount, elapsed, linesPerSec)
	fmt.Printf("Found %d events from %d unique authors\n", eventCount, len(uniqueAuthors))
	fmt.Printf("Output written to directory: %s\n", *outputDir)

	// Print top 10 authors by event count
	stats := fileWriter.GetStats()
	fmt.Println("\nTop authors by event count:")
	
	// Convert map to slice for sorting
	type authorCount struct {
		pubkey string
		count  int
	}
	authorCounts := make([]authorCount, 0, len(stats))
	for pubkey, count := range stats {
		authorCounts = append(authorCounts, authorCount{pubkey, count})
	}
	
	// Sort by count (descending)
	for i := 0; i < len(authorCounts)-1; i++ {
		for j := i + 1; j < len(authorCounts); j++ {
			if authorCounts[i].count < authorCounts[j].count {
				authorCounts[i], authorCounts[j] = authorCounts[j], authorCounts[i]
			}
		}
	}
	
	// Print top 10 or all if less than 10
	limit := 10
	if len(authorCounts) < limit {
		limit = len(authorCounts)
	}
	for i := 0; i < limit; i++ {
		fmt.Printf("%s: %d events\n", authorCounts[i].pubkey, authorCounts[i].count)
	}
}
