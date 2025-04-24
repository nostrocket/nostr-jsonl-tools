package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"path/filepath"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

// Node represents a pubkey and its followers
type Node struct {
	Pubkey    string            `json:"pubkey"`
	Npub      string            `json:"npub,omitempty"`
	Following map[string]*Node  `json:"following"`
}

// Graph represents the entire follow graph
type Graph struct {
	Nodes map[string]*Node
}

// NewGraph creates a new empty graph
func NewGraph() *Graph {
	return &Graph{
		Nodes: make(map[string]*Node),
	}
}

// GetOrCreateNode gets a node from the graph or creates it if it doesn't exist
func (g *Graph) GetOrCreateNode(pubkey string) *Node {
	if node, exists := g.Nodes[pubkey]; exists {
		return node
	}
	
	// Create npub for display
	npub, err := nip19.EncodePublicKey(pubkey)
	if err != nil {
		npub = ""
	}
	
	node := &Node{
		Pubkey:    pubkey,
		Npub:      npub,
		Following: make(map[string]*Node),
	}
	g.Nodes[pubkey] = node
	return node
}

// AddFollowing adds a following relationship to the graph
func (g *Graph) AddFollowing(pubkey, followingPubkey string) {
	node := g.GetOrCreateNode(pubkey)
	followingNode := g.GetOrCreateNode(followingPubkey)
	node.Following[followingPubkey] = followingNode
}

// PrintGraph prints the graph recursively starting from a root pubkey
func (g *Graph) PrintGraph(rootPubkey string, depth int, useNpub bool, maxDepth int) {
	node, exists := g.Nodes[rootPubkey]
	if !exists {
		fmt.Printf("%s%s (not found in graph)\n", strings.Repeat("  ", depth), rootPubkey)
		return
	}

	displayKey := node.Pubkey
	if useNpub && node.Npub != "" {
		displayKey = node.Npub
	}

	fmt.Printf("%s%s (%d following)\n", strings.Repeat("  ", depth), displayKey, len(node.Following))
	
	// To avoid infinite recursion in case of cycles
	visited := make(map[string]bool)
	g.printNodeRecursive(node, depth+1, visited, useNpub, maxDepth)
}

// PrintGraphToWriter prints the graph recursively starting from a root pubkey to a writer
func (g *Graph) PrintGraphToWriter(rootPubkey string, depth int, useNpub bool, maxDepth int, writer io.Writer) {
	node, exists := g.Nodes[rootPubkey]
	if !exists {
		fmt.Fprintf(writer, "%s%s (not found in graph)\n", strings.Repeat("  ", depth), rootPubkey)
		return
	}

	displayKey := node.Pubkey
	if useNpub && node.Npub != "" {
		displayKey = node.Npub
	}

	fmt.Fprintf(writer, "%s%s (%d following)\n", strings.Repeat("  ", depth), displayKey, len(node.Following))
	
	// To avoid infinite recursion in case of cycles
	visited := make(map[string]bool)
	g.printNodeRecursiveToWriter(node, depth+1, visited, useNpub, maxDepth, writer)
}

// printNodeRecursive prints a node and its followers recursively
func (g *Graph) printNodeRecursive(node *Node, depth int, visited map[string]bool, useNpub bool, maxDepth int) {
	// Stop recursion if we've reached the maximum depth
	if maxDepth > 0 && depth > maxDepth {
		fmt.Printf("%s... (max depth reached)\n", strings.Repeat("  ", depth))
		return
	}

	if visited[node.Pubkey] {
		displayKey := node.Pubkey
		if useNpub && node.Npub != "" {
			displayKey = node.Npub
		}
		fmt.Printf("%s%s (already visited)\n", strings.Repeat("  ", depth), displayKey)
		return
	}
	
	visited[node.Pubkey] = true
	
	for _, followingNode := range node.Following {
		displayKey := followingNode.Pubkey
		if useNpub && followingNode.Npub != "" {
			displayKey = followingNode.Npub
		}
		fmt.Printf("%s%s\n", strings.Repeat("  ", depth), displayKey)
		g.printNodeRecursive(followingNode, depth+1, visited, useNpub, maxDepth)
	}
}

// printNodeRecursiveToWriter prints a node and its followers recursively to a writer
func (g *Graph) printNodeRecursiveToWriter(node *Node, depth int, visited map[string]bool, useNpub bool, maxDepth int, writer io.Writer) {
	// Stop recursion if we've reached the maximum depth
	if maxDepth > 0 && depth > maxDepth {
		fmt.Fprintf(writer, "%s... (max depth reached)\n", strings.Repeat("  ", depth))
		return
	}

	if visited[node.Pubkey] {
		displayKey := node.Pubkey
		if useNpub && node.Npub != "" {
			displayKey = node.Npub
		}
		fmt.Fprintf(writer, "%s%s (already visited)\n", strings.Repeat("  ", depth), displayKey)
		return
	}
	
	visited[node.Pubkey] = true
	
	for _, followingNode := range node.Following {
		displayKey := followingNode.Pubkey
		if useNpub && followingNode.Npub != "" {
			displayKey = followingNode.Npub
		}
		fmt.Fprintf(writer, "%s%s\n", strings.Repeat("  ", depth), displayKey)
		g.printNodeRecursiveToWriter(followingNode, depth+1, visited, useNpub, maxDepth, writer)
	}
}

// ExportJSON exports the graph as JSON starting from a root pubkey
func (g *Graph) ExportJSON(rootPubkey string, maxDepth int) ([]byte, error) {
	node, exists := g.Nodes[rootPubkey]
	if !exists {
		return json.Marshal(map[string]string{
			"error": fmt.Sprintf("Pubkey %s not found in graph", rootPubkey),
		})
	}

	// Create a clean tree structure for JSON export
	jsonTree := g.buildJSONTree(node, make(map[string]bool), 0, maxDepth)
	return json.MarshalIndent(jsonTree, "", "  ")
}

// ExportSimpleJSON exports the graph as a simple JSON structure with only pubkeys
func (g *Graph) ExportSimpleJSON(rootPubkey string, maxDepth int) ([]byte, error) {
	node, exists := g.Nodes[rootPubkey]
	if !exists {
		return json.Marshal(map[string]string{
			"error": fmt.Sprintf("Pubkey %s not found in graph", rootPubkey),
		})
	}

	// Create a simple tree structure for JSON export
	jsonTree := g.buildSimpleJSONTree(node, make(map[string]bool), 0, maxDepth)
	return json.MarshalIndent(jsonTree, "", "  ")
}

// ExportPubkeyList exports a line-separated list of all pubkeys in the graph
// starting from the root pubkey
func (g *Graph) ExportPubkeyList(rootPubkey string, maxDepth int) ([]byte, error) {
	node, exists := g.Nodes[rootPubkey]
	if !exists {
		return []byte(fmt.Sprintf("Error: Pubkey %s not found in graph", rootPubkey)), nil
	}

	// Use a map to track unique pubkeys
	pubkeys := make(map[string]bool)
	g.collectPubkeys(node, pubkeys, make(map[string]bool), 0, maxDepth)
	
	// Convert the map to a line-separated list
	var result strings.Builder
	for pubkey := range pubkeys {
		result.WriteString(pubkey)
		result.WriteString("\n")
	}
	
	return []byte(result.String()), nil
}

// collectPubkeys collects all unique pubkeys in the graph
func (g *Graph) collectPubkeys(node *Node, pubkeys map[string]bool, visited map[string]bool, depth int, maxDepth int) {
	// Stop recursion if we've reached the maximum depth
	if maxDepth > 0 && depth > maxDepth {
		return
	}

	if visited[node.Pubkey] {
		return
	}
	
	visited[node.Pubkey] = true
	pubkeys[node.Pubkey] = true
	
	for _, followingNode := range node.Following {
		pubkeys[followingNode.Pubkey] = true
		g.collectPubkeys(followingNode, pubkeys, visited, depth+1, maxDepth)
	}
}

// buildJSONTree builds a clean tree structure for JSON export
func (g *Graph) buildJSONTree(node *Node, visited map[string]bool, depth int, maxDepth int) map[string]interface{} {
	// Stop recursion if we've reached the maximum depth
	if maxDepth > 0 && depth > maxDepth {
		return map[string]interface{}{
			"pubkey": node.Pubkey,
			"npub":   node.Npub,
			"note":   "max depth reached",
		}
	}

	if visited[node.Pubkey] {
		return map[string]interface{}{
			"pubkey": node.Pubkey,
			"npub":   node.Npub,
			"note":   "already visited",
		}
	}
	
	visited[node.Pubkey] = true
	
	result := map[string]interface{}{
		"pubkey":    node.Pubkey,
		"npub":      node.Npub,
		"following": []interface{}{},
	}
	
	following := []interface{}{}
	for _, followingNode := range node.Following {
		following = append(following, g.buildJSONTree(followingNode, visited, depth+1, maxDepth))
	}
	
	result["following"] = following
	return result
}

// buildSimpleJSONTree builds a simple tree structure for JSON export with only pubkeys
func (g *Graph) buildSimpleJSONTree(node *Node, visited map[string]bool, depth int, maxDepth int) map[string]interface{} {
	// Stop recursion if we've reached the maximum depth
	if maxDepth > 0 && depth > maxDepth {
		return map[string]interface{}{}
	}

	if visited[node.Pubkey] {
		return map[string]interface{}{}
	}
	
	visited[node.Pubkey] = true
	
	result := make(map[string]interface{})
	
	for _, followingNode := range node.Following {
		result[followingNode.Pubkey] = g.buildSimpleJSONTree(followingNode, visited, depth+1, maxDepth)
	}
	
	return result
}

// Stats returns statistics about the graph
func (g *Graph) Stats() map[string]interface{} {
	stats := make(map[string]interface{})
	stats["total_nodes"] = len(g.Nodes)
	
	// Find the node with the most followers
	maxFollowing := 0
	maxFollowingPubkey := ""
	
	for pubkey, node := range g.Nodes {
		if len(node.Following) > maxFollowing {
			maxFollowing = len(node.Following)
			maxFollowingPubkey = pubkey
		}
	}
	
	stats["max_following"] = maxFollowing
	stats["max_following_pubkey"] = maxFollowingPubkey
	
	// Debug print directly to stderr to ensure it's visible
	fmt.Fprintf(os.Stderr, "Debug - Stats: total_nodes=%d, max_following=%d, max_following_pubkey=%s\n", 
		len(g.Nodes), maxFollowing, maxFollowingPubkey)
	
	return stats
}

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
		fmt.Println("Usage: go run main.go -file <jsonl_file> -pubkey <root_pubkey> [-json] [-npub] [-max-depth N] [-stats]")
		fmt.Println("   or: go run main.go <jsonl_file> <root_pubkey>")
		os.Exit(1)
	}

	jsonlFile := *jsonlFilePtr
	rootPubkey := *rootPubkeyPtr

	// Convert npub to hex if needed
	if strings.HasPrefix(rootPubkey, "npub") {
		prefix, decoded, err := nip19.Decode(rootPubkey)
		if err != nil {
			fmt.Printf("Error decoding npub: %v\n", err)
			os.Exit(1)
		}
		if prefix != "npub" {
			fmt.Printf("Expected npub prefix, got %s\n", prefix)
			os.Exit(1)
		}
		rootPubkey = decoded.(string)
	}

	// Create a new graph
	graph := NewGraph()

	// Open the JSONL file
	file, err := os.Open(jsonlFile)
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
	
	fmt.Printf("Processing file %s (%d bytes)...\n", jsonlFile, fileSize)
	
	// Read the file line by line with a larger buffer
	scanner := bufio.NewScanner(file)
	// Increase the buffer size to handle large lines
	const maxCapacity = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)
	
	lineCount := 0
	eventCount := 0
	contactListCount := 0
	lastProgressTime := time.Now()
	
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++
		
		// Show progress every second
		if time.Since(lastProgressTime) > time.Second {
			fmt.Printf("\rProcessed %d lines, %d events, %d contact lists...", lineCount, eventCount, contactListCount)
			lastProgressTime = time.Now()
		}
		
		// Parse the JSON line into an event
		var event nostr.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Skip invalid JSON
			continue
		}
		
		eventCount++

		// We're only interested in kind 3 events (contact lists)
		if event.Kind != 3 {
			continue
		}
		
		contactListCount++

		pubkey := event.PubKey
		
		// Process the tags to find p tags (followed pubkeys)
		for _, tag := range event.Tags {
			if len(tag) >= 2 && tag[0] == "p" {
				followedPubkey := tag[1]
				graph.AddFollowing(pubkey, followedPubkey)
			}
		}
	}
	
	fmt.Printf("\rProcessed %d lines, %d events, %d contact lists.\n", lineCount, eventCount, contactListCount)

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading file: %v\n", err)
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
