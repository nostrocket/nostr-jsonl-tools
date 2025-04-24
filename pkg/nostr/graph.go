package nostr

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

// Node represents a pubkey and its followers
type Node struct {
	Pubkey    string           `json:"pubkey"`
	Npub      string           `json:"npub,omitempty"`
	Following map[string]*Node `json:"following"`
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

	return stats
}

// BuildGraphFromJSONL builds a graph from a JSONL file containing Nostr events
func BuildGraphFromJSONL(jsonlFile string) (*Graph, error) {
	// Create a new graph
	graph := NewGraph()

	// Open the JSONL file
	file, err := os.Open(jsonlFile)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	// Get file info for progress reporting
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("error getting file info: %v", err)
	}
	fileSize := fileInfo.Size()

	fmt.Fprintf(os.Stderr, "Processing file %s (%d bytes)...\n", jsonlFile, fileSize)

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
			fmt.Fprintf(os.Stderr, "\rProcessed %d lines, %d events, %d contact lists...", lineCount, eventCount, contactListCount)
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

	fmt.Fprintf(os.Stderr, "\rProcessed %d lines, %d events, %d contact lists.\n", lineCount, eventCount, contactListCount)

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return graph, nil
}

// DecodeNpubIfNeeded decodes an npub to a hex pubkey if needed
func DecodeNpubIfNeeded(pubkey string) (string, error) {
	if strings.HasPrefix(pubkey, "npub") {
		prefix, decoded, err := nip19.Decode(pubkey)
		if err != nil {
			return "", fmt.Errorf("error decoding npub: %v", err)
		}
		if prefix != "npub" {
			return "", fmt.Errorf("expected npub prefix, got %s", prefix)
		}
		return decoded.(string), nil
	}
	return pubkey, nil
}
