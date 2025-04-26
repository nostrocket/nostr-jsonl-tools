package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// NostrEvent represents a simplified Nostr event structure
type NostrEvent struct {
	ID      string `json:"id"`
	Pubkey  string `json:"pubkey"`
	Created int64  `json:"created_at"`
	Kind    int    `json:"kind"`
	Content string `json:"content"`
}

func main() {
	// Parse command line flags
	inputFile := flag.String("file", "", "Input JSONL file")
	pubkeysFile := flag.String("pubkeys", "", "File containing line-separated pubkeys to filter by")
	dbFile := flag.String("db", "nostr_content.db", "SQLite database file")
	flag.Parse()

	// Check if required files are provided
	if *inputFile == "" || *pubkeysFile == "" {
		fmt.Println("Error: Input file and pubkeys file are required.")
		fmt.Println("Usage: ./content-extractor -file <jsonl_file> -pubkeys <pubkeys_file> [-db <database_file>]")
		os.Exit(1)
	}

	// Load pubkeys from file into a map for quick lookup
	pubkeys, err := loadPubkeys(*pubkeysFile)
	if err != nil {
		fmt.Printf("Error loading pubkeys: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Loaded %d pubkeys to filter by\n", len(pubkeys))

	// Initialize SQLite database
	db, err := initDatabase(*dbFile)
	if err != nil {
		fmt.Printf("Error initializing database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

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
	matchCount := 0
	startTime := time.Now()
	lastReportTime := startTime

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

		// Check if this pubkey is in our filter list
		if pubkeys[event.Pubkey] {
			// Store the content in the database
			if err := storeContent(db, event); err != nil {
				fmt.Printf("Error storing content: %v\n", err)
				continue
			}
			matchCount++
		}

		// Print progress every 100,000 lines or 5 seconds
		currentTime := time.Now()
		if lineCount%100000 == 0 || currentTime.Sub(lastReportTime) > 5*time.Second {
			elapsed := currentTime.Sub(startTime).Seconds()
			linesPerSec := float64(lineCount) / elapsed
			fmt.Printf("Processed %d lines (%.2f lines/sec), matched %d events\n", 
				lineCount, linesPerSec, matchCount)
			lastReportTime = currentTime
		}
	}

	// Print final statistics
	elapsed := time.Since(startTime).Seconds()
	linesPerSec := float64(lineCount) / elapsed
	fmt.Printf("\nCompleted processing %d lines in %.2f seconds (%.2f lines/sec)\n", 
		lineCount, elapsed, linesPerSec)
	fmt.Printf("Matched and stored content for %d events\n", matchCount)
	fmt.Printf("Database: %s\n", *dbFile)
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

// initDatabase initializes the SQLite database
func initDatabase(dbFile string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return nil, err
	}

	// Create tables if they don't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS authors (
			pubkey TEXT PRIMARY KEY,
			content TEXT,
			event_count INTEGER DEFAULT 0,
			last_updated TIMESTAMP
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// storeContent stores or updates content in the database
func storeContent(db *sql.DB, event NostrEvent) error {
	// Skip empty content
	if event.Content == "" {
		return nil
	}

	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Check if the pubkey already exists
	var exists bool
	var currentContent string
	var eventCount int
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM authors WHERE pubkey = ?), content, event_count FROM authors WHERE pubkey = ?", 
		event.Pubkey, event.Pubkey).Scan(&exists, &currentContent, &eventCount)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	now := time.Now().Format(time.RFC3339)
	
	if exists {
		// Append the new content to the existing content
		newContent := currentContent
		if newContent != "" {
			newContent += "\n\n"
		}
		newContent += fmt.Sprintf("[Event ID: %s, Created: %d]\n%s", 
			event.ID, event.Created, event.Content)
		
		// Update the record
		_, err = tx.Exec("UPDATE authors SET content = ?, event_count = ?, last_updated = ? WHERE pubkey = ?", 
			newContent, eventCount+1, now, event.Pubkey)
	} else {
		// Insert a new record
		_, err = tx.Exec("INSERT INTO authors (pubkey, content, event_count, last_updated) VALUES (?, ?, ?, ?)", 
			event.Pubkey, fmt.Sprintf("[Event ID: %s, Created: %d]\n%s", event.ID, event.Created, event.Content), 
			1, now)
	}
	
	if err != nil {
		return err
	}

	// Commit the transaction
	return tx.Commit()
}
