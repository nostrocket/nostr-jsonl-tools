package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nbd-wtf/go-nostr"
	pb "nostr-follow-graph/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GraphServiceClient is a client for our graph service
type GraphServiceClient struct {
	conn *grpc.ClientConn
	client pb.GraphServiceClient
	connected bool
}

// NewGraphServiceClient creates a new GraphServiceClient
func NewGraphServiceClient(address string) (*GraphServiceClient, error) {
	// Set up a connection to the server
	fmt.Printf("Connecting to graph server at %s...\n", address)
	
	// Set a shorter timeout for the initial connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	conn, err := grpc.DialContext(ctx, address, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock()) // Block until the connection is established or times out
	
	if err != nil {
		return nil, fmt.Errorf("failed to connect to graph server: %v", err)
	}
	
	// Create the gRPC client
	client := pb.NewGraphServiceClient(conn)
	
	// Test the connection with a ping
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer pingCancel()
	
	_, err = client.Ping(pingCtx, &pb.PingRequest{})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping graph server: %v", err)
	}
	
	fmt.Println("Successfully connected to graph server")
	
	return &GraphServiceClient{
		conn: conn,
		client: client,
		connected: true,
	}, nil
}

// Close closes the GraphServiceClient
func (c *GraphServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// StoreFollow stores a follow relationship
func (c *GraphServiceClient) StoreFollow(ctx context.Context, follower, followed string) error {
	// Make the actual gRPC call to the server
	_, err := c.client.StoreFollow(ctx, &pb.StoreFollowRequest{
		Follower: follower,
		Followed: followed,
	})
	
	if err != nil {
		return fmt.Errorf("error storing follow relationship: %v", err)
	}
	
	fmt.Printf("Storing follow relationship: %s -> %s\n", follower, followed)
	return nil
}

// GetFollowing gets all pubkeys that a user is following
func (c *GraphServiceClient) GetFollowing(ctx context.Context, pubkey string) ([]string, error) {
	// Make the actual gRPC call to the server
	resp, err := c.client.GetFollowing(ctx, &pb.GetFollowingRequest{
		Pubkey: pubkey,
	})
	
	if err != nil {
		return nil, fmt.Errorf("error getting following: %v", err)
	}
	
	return resp.Pubkeys, nil
}

// GetFollowers gets all pubkeys that follow a user
func (c *GraphServiceClient) GetFollowers(ctx context.Context, pubkey string) ([]string, error) {
	// Make the actual gRPC call to the server
	resp, err := c.client.GetFollowers(ctx, &pb.GetFollowersRequest{
		Pubkey: pubkey,
	})
	
	if err != nil {
		return nil, fmt.Errorf("error getting followers: %v", err)
	}
	
	return resp.Pubkeys, nil
}

// Reset resets the database
func (c *GraphServiceClient) Reset(ctx context.Context) error {
	// Make the actual gRPC call to the server
	_, err := c.client.Reset(ctx, &pb.ResetRequest{})
	
	if err != nil {
		return fmt.Errorf("error resetting database: %v", err)
	}
	
	fmt.Println("Resetting graph database")
	return nil
}

// Ping checks if the server is responsive
func (c *GraphServiceClient) Ping(ctx context.Context) error {
	// Make the actual gRPC call to the server
	_, err := c.client.Ping(ctx, &pb.PingRequest{})
	
	if err != nil {
		return fmt.Errorf("error pinging server: %v", err)
	}
	
	return nil
}

func main() {
	// Parse command line flags
	filePath := flag.String("file", "", "Path to JSONL file containing Nostr events")
	pubkey := flag.String("pubkey", "", "Pubkey to query (optional)")
	dgraphAddress := flag.String("dgraph", "localhost:9080", "Address of the graph server")
	resetDB := flag.Bool("reset", false, "Reset the database before importing")
	noProgress := flag.Bool("no-progress", false, "Disable progress updates")
	queryMode := flag.Bool("query", false, "Run queries without importing data")
	retries := flag.Int("retries", 3, "Number of retries for failed operations")
	// We're not using these variables yet, but keeping them for future implementation
	_ = flag.Int("depth", 1, "Depth of query (number of hops)")
	_ = flag.Int("limit", 100, "Limit number of results")
	flag.Parse()

	// Create a new GraphServiceClient
	client, err := NewGraphServiceClient(*dgraphAddress)
	if err != nil {
		log.Fatalf("Error creating GraphServiceClient: %v", err)
	}
	defer client.Close()

	// Create a context
	ctx := context.Background()

	// Verify connection with a ping
	if err := client.Ping(ctx); err != nil {
		log.Fatalf("Error connecting to graph server: %v", err)
	}

	// Reset the database if requested
	if *resetDB {
		if err := client.Reset(ctx); err != nil {
			log.Fatalf("Error resetting database: %v", err)
		}
		fmt.Println("Database reset successfully")
	}

	// If query mode is enabled, run queries without importing data
	if *queryMode {
		if *pubkey == "" {
			log.Fatalf("Pubkey is required for query mode")
		}

		// Get following
		following, err := client.GetFollowing(ctx, *pubkey)
		if err != nil {
			log.Fatalf("Error getting following: %v", err)
		}
		fmt.Printf("Following (%d):\n", len(following))
		for _, p := range following {
			fmt.Printf("  %s\n", p)
		}

		// Get followers
		followers, err := client.GetFollowers(ctx, *pubkey)
		if err != nil {
			log.Fatalf("Error getting followers: %v", err)
		}
		fmt.Printf("Followers (%d):\n", len(followers))
		for _, p := range followers {
			fmt.Printf("  %s\n", p)
		}

		return
	}

	// Check if file path is provided
	if *filePath == "" {
		log.Fatalf("File path is required")
	}

	// Open the file
	file, err := os.Open(*filePath)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer file.Close()

	// Get file info for progress reporting
	fileInfo, err := file.Stat()
	if err != nil {
		log.Fatalf("Error getting file info: %v", err)
	}
	fileSize := fileInfo.Size()
	fmt.Printf("Processing file %s (%.2f GB)...\n", *filePath, float64(fileSize)/(1024*1024*1024))

	// Process the file with a larger buffer
	scanner := bufio.NewScanner(file)
	
	// Increase the buffer size to handle large lines
	const maxCapacity = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)
	
	lineCount := 0
	eventCount := 0
	followCount := 0
	successCount := 0
	errorCount := 0
	lastProgressTime := time.Now()
	startTime := time.Now()
	
	// Track unique relationships to avoid duplicates
	seenRelationships := make(map[string]bool)

	// Process each line
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		// Show progress every second if not disabled
		if !*noProgress && time.Since(lastProgressTime) > time.Second {
			elapsed := time.Since(startTime)
			linesPerSecond := float64(lineCount) / elapsed.Seconds()
			bytesRead, _ := file.Seek(0, os.SEEK_CUR)
			percentComplete := float64(bytesRead) * 100.0 / float64(fileSize)
			fmt.Printf("\rProcessed %d lines, %d events, %d follows, %d stored, %d errors (%.1f lines/sec, %.1f%% complete)...", 
				lineCount, eventCount, followCount, successCount, errorCount, linesPerSecond, percentComplete)
			lastProgressTime = time.Now()
		}

		// Parse the JSON
		var event nostr.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Skip invalid JSON but log the error if progress updates are enabled
			if !*noProgress {
				fmt.Printf("\nError parsing JSON on line %d: %v\n", lineCount, err)
			}
			continue
		}

		// Only process kind 3 events (contact lists)
		if event.Kind != 3 {
			continue
		}

		eventCount++
		pubkey := event.PubKey

		// Process tags
		for _, tag := range event.Tags {
			if len(tag) >= 2 && tag[0] == "p" {
				followedPubkey := tag[1]
				
				// Create a unique key for this relationship
				relationshipKey := pubkey + ":" + followedPubkey
				
				// Skip if we've already seen this relationship
				if seenRelationships[relationshipKey] {
					continue
				}
				
				// Mark as seen
				seenRelationships[relationshipKey] = true
				
				followCount++

				// Store the relationship immediately with retries
				stored := false
				var lastErr error
				
				for attempt := 0; attempt < *retries; attempt++ {
					// Create a context with timeout for each attempt
					storeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
					
					err := client.StoreFollow(storeCtx, pubkey, followedPubkey)
					cancel() // Always cancel the context to avoid leaks
					
					if err == nil {
						stored = true
						successCount++
						break
					}
					
					lastErr = err
					
					// Exponential backoff
					delay := time.Duration(1<<uint(attempt)) * 100 * time.Millisecond
					if delay > 2*time.Second {
						delay = 2 * time.Second
					}
					
					if !*noProgress {
						fmt.Printf("\nError storing follow relationship (attempt %d/%d): %v. Retrying in %v...\n", 
							attempt+1, *retries, err, delay)
					}
					time.Sleep(delay)
				}
				
				if !stored {
					errorCount++
					if !*noProgress {
						fmt.Printf("\nFailed to store relationship after %d attempts: %v\n", *retries, lastErr)
					}
				}
			}
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	// Print final statistics
	fmt.Printf("\nProcessed %d lines, %d events, %d follow relationships\n", lineCount, eventCount, followCount)
	fmt.Printf("Successfully stored: %d, Errors: %d\n", successCount, errorCount)
	fmt.Printf("Total processing time: %v\n", time.Since(startTime))
}
