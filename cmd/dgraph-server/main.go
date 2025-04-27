package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dgraph-io/badger/v3"
	pb "nostr-follow-graph/proto"
	"google.golang.org/grpc"
)

// Default port for our embedded graph server
const (
	defaultGrpcPort = 9080
)

// GraphServer implements a simple graph database server
type GraphServer struct {
	db         *badger.DB
	dataDir    string
	grpcServer *grpc.Server
	mu         sync.RWMutex
	stats      *ServerStats
}

// ServerStats tracks server statistics
type ServerStats struct {
	mu                sync.RWMutex
	followsStored     int64
	followsRetrieved  int64
	startTime         time.Time
	lastActivityTime  time.Time
	totalFollows      int64
	totalUsers        int64
	totalFollowQueries int64
}

// NewServerStats creates a new ServerStats
func NewServerStats() *ServerStats {
	return &ServerStats{
		startTime:        time.Now(),
		lastActivityTime: time.Now(),
	}
}

// RecordFollowStored records a follow relationship being stored
func (s *ServerStats) RecordFollowStored() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.followsStored++
	s.lastActivityTime = time.Now()
}

// RecordFollowRetrieved records a follow relationship being retrieved
func (s *ServerStats) RecordFollowRetrieved() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.followsRetrieved++
	s.lastActivityTime = time.Now()
}

// RecordFollowQuery records a follow query
func (s *ServerStats) RecordFollowQuery() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalFollowQueries++
	s.lastActivityTime = time.Now()
}

// GetStats returns the current statistics
func (s *ServerStats) GetStats() (int64, int64, time.Duration, time.Duration) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.followsStored, s.followsRetrieved, time.Since(s.startTime), time.Since(s.lastActivityTime)
}

// UpdateTotals updates the total counts
func (s *ServerStats) UpdateTotals(users, follows int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalUsers = users
	s.totalFollows = follows
}

// GetTotals returns the total counts
func (s *ServerStats) GetTotals() (int64, int64, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalUsers, s.totalFollows, s.totalFollowQueries
}

// NewGraphServer creates a new GraphServer
func NewGraphServer(dataDir string) (*GraphServer, error) {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("error creating data directory: %v", err)
	}

	// Open Badger database
	opts := badger.DefaultOptions(dataDir).
		WithLogger(nil) // Disable logging

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("error opening Badger database: %v", err)
	}

	return &GraphServer{
		db:      db,
		dataDir: dataDir,
		stats:   NewServerStats(),
	}, nil
}

// Close closes the GraphServer
func (s *GraphServer) Close() error {
	return s.db.Close()
}

// StartGRPC starts the gRPC server
func (s *GraphServer) StartGRPC(port int, verbose bool) error {
	// Create a gRPC server
	s.grpcServer = grpc.NewServer()

	// Register our services
	pb.RegisterGraphServiceServer(s.grpcServer, &graphServiceServer{server: s})

	// Start the gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("error starting gRPC server: %v", err)
	}

	// Start the server in a goroutine
	go func() {
		if verbose {
			log.Printf("Starting gRPC server on port %d...\n", port)
		} else {
			fmt.Printf("Starting gRPC server on port %d...\n", port)
		}
		
		if err := s.grpcServer.Serve(lis); err != nil {
			log.Fatalf("Error serving gRPC: %v", err)
		}
	}()

	return nil
}

// StartStatsReporter starts a goroutine that reports statistics periodically
func (s *GraphServer) StartStatsReporter(verbose bool) {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Update total counts
				users, follows := s.CountUsersAndFollows()
				s.stats.UpdateTotals(users, follows)

				// Get stats
				stored, retrieved, uptime, lastActivity := s.stats.GetStats()
				totalUsers, totalFollows, totalQueries := s.stats.GetTotals()

				// Print stats
				fmt.Println("\n=== Server Statistics ===")
				fmt.Printf("Uptime: %v\n", uptime.Round(time.Second))
				fmt.Printf("Last activity: %v ago\n", lastActivity.Round(time.Second))
				fmt.Printf("Total users: %d\n", totalUsers)
				fmt.Printf("Total follow relationships: %d\n", totalFollows)
				fmt.Printf("Follows stored since startup: %d\n", stored)
				fmt.Printf("Follows retrieved since startup: %d\n", retrieved)
				fmt.Printf("Total queries since startup: %d\n", totalQueries)
				fmt.Println("=========================")
			}
		}
	}()
}

// CountUsersAndFollows counts the total number of users and follow relationships
func (s *GraphServer) CountUsersAndFollows() (int64, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var users int64
	var follows int64
	userMap := make(map[string]bool)

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("follows:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			k := item.Key()
			
			// Extract follower and followed from key
			parts := strings.Split(string(k), ":")
			if len(parts) == 3 {
				follower := parts[1]
				followed := parts[2]
				
				// Count unique users
				if !userMap[follower] {
					userMap[follower] = true
					users++
				}
				if !userMap[followed] {
					userMap[followed] = true
					users++
				}
				
				follows++
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("Error counting users and follows: %v", err)
	}

	return users, follows
}

// StopGRPC stops the gRPC server
func (s *GraphServer) StopGRPC() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}

// StoreFollow stores a follow relationship
func (s *GraphServer) StoreFollow(follower, followed string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("follows:%s:%s", follower, followed)
	err := s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), []byte{1})
	})

	if err == nil {
		s.stats.RecordFollowStored()
	}

	return err
}

// GetFollowing gets all pubkeys that a user is following
func (s *GraphServer) GetFollowing(pubkey string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.stats.RecordFollowQuery()

	var following []string
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte(fmt.Sprintf("follows:%s:", pubkey))
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			k := item.Key()
			
			// Extract followed pubkey from key
			parts := strings.Split(string(k), ":")
			if len(parts) == 3 {
				followed := parts[2]
				following = append(following, followed)
				s.stats.RecordFollowRetrieved()
			}
		}
		return nil
	})

	return following, err
}

// GetFollowers gets all pubkeys that follow a user
func (s *GraphServer) GetFollowers(pubkey string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.stats.RecordFollowQuery()

	var followers []string
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		// Scan all follow relationships
		prefix := []byte("follows:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			k := item.Key()
			
			// Extract follower and followed from key
			parts := strings.Split(string(k), ":")
			if len(parts) == 3 && parts[2] == pubkey {
				follower := parts[1]
				followers = append(followers, follower)
				s.stats.RecordFollowRetrieved()
			}
		}
		return nil
	})

	return followers, err
}

// Reset resets the database
func (s *GraphServer) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.DropAll()
}

// graphServiceServer implements the GraphService gRPC service
type graphServiceServer struct {
	pb.UnimplementedGraphServiceServer
	server *GraphServer
}

// StoreFollow implements the StoreFollow RPC
func (s *graphServiceServer) StoreFollow(ctx context.Context, req *pb.StoreFollowRequest) (*pb.StoreFollowResponse, error) {
	err := s.server.StoreFollow(req.Follower, req.Followed)
	return &pb.StoreFollowResponse{Success: err == nil}, err
}

// GetFollowing implements the GetFollowing RPC
func (s *graphServiceServer) GetFollowing(ctx context.Context, req *pb.GetFollowingRequest) (*pb.GetFollowingResponse, error) {
	following, err := s.server.GetFollowing(req.Pubkey)
	return &pb.GetFollowingResponse{Pubkeys: following}, err
}

// GetFollowers implements the GetFollowers RPC
func (s *graphServiceServer) GetFollowers(ctx context.Context, req *pb.GetFollowersRequest) (*pb.GetFollowersResponse, error) {
	followers, err := s.server.GetFollowers(req.Pubkey)
	return &pb.GetFollowersResponse{Pubkeys: followers}, err
}

// Reset implements the Reset RPC
func (s *graphServiceServer) Reset(ctx context.Context, req *pb.ResetRequest) (*pb.ResetResponse, error) {
	err := s.server.Reset()
	return &pb.ResetResponse{Success: err == nil}, err
}

// Ping implements the Ping RPC
func (s *graphServiceServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{Success: true}, nil
}

func main() {
	// Parse command line flags
	dataDir := flag.String("data", "graph-data", "Directory to store graph data")
	grpcPort := flag.Int("port", defaultGrpcPort, "Port for gRPC server")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	// Create a new GraphServer
	server, err := NewGraphServer(*dataDir)
	if err != nil {
		log.Fatalf("Error creating GraphServer: %v", err)
	}
	defer server.Close()

	// Start the gRPC server
	if err := server.StartGRPC(*grpcPort, *verbose); err != nil {
		log.Fatalf("Error starting gRPC server: %v", err)
	}

	// Start the stats reporter
	server.StartStatsReporter(*verbose)

	// Print server info
	fmt.Println("\nGraph database server is running!")
	fmt.Println("================================")
	fmt.Printf("Data directory: %s\n", *dataDir)
	fmt.Printf("gRPC port: %d\n", *grpcPort)
	fmt.Println("================================")
	fmt.Println("Use Ctrl+C to stop the server")
	fmt.Println("To connect to this server with dgraph-follow, use:")
	fmt.Printf("./dgraph-follow -dgraph localhost:%d ...\n", *grpcPort)
	fmt.Println("================================")

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	// Shutdown
	fmt.Println("\nShutting down graph database server...")
	server.StopGRPC()
	fmt.Println("Graph database server stopped")
}
