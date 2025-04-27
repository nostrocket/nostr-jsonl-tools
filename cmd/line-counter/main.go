package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FileStats holds statistics about a file
type FileStats struct {
	Path      string
	LineCount int64
	Size      int64
}

func main() {
	// Parse command line flags
	inputPath := flag.String("path", "", "Path to file or directory to count lines in")
	recursive := flag.Bool("recursive", false, "Recursively count lines in directories")
	workers := flag.Int("workers", 4, "Number of worker goroutines for parallel processing")
	sortBy := flag.String("sort", "none", "Sort output by: none, name, lines, size")
	includePattern := flag.String("include", "", "Only include files matching pattern (e.g., '*.jsonl')")
	excludePattern := flag.String("exclude", "", "Exclude files matching pattern (e.g., '*.git*')")
	flag.Parse()

	// Check if input path is provided
	if *inputPath == "" {
		fmt.Println("Error: Input path is required.")
		fmt.Println("Usage: ./line-counter -path <file_or_directory> [-recursive] [-workers <num>] [-sort <none|name|lines|size>] [-include <pattern>] [-exclude <pattern>]")
		os.Exit(1)
	}

	// Check if the path exists
	fileInfo, err := os.Stat(*inputPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	startTime := time.Now()
	var stats []FileStats

	// Process a single file or a directory
	if !fileInfo.IsDir() {
		// Count lines in a single file
		lineCount, size, err := countLinesInFile(*inputPath)
		if err != nil {
			fmt.Printf("Error counting lines in %s: %v\n", *inputPath, err)
			os.Exit(1)
		}
		stats = append(stats, FileStats{
			Path:      *inputPath,
			LineCount: lineCount,
			Size:      size,
		})
	} else {
		// Count lines in a directory
		stats, err = countLinesInDirectory(*inputPath, *recursive, *workers, *includePattern, *excludePattern)
		if err != nil {
			fmt.Printf("Error counting lines in directory: %v\n", err)
			os.Exit(1)
		}
	}

	// Sort the results if requested
	sortStats(stats, *sortBy)

	// Print the results
	var totalLines int64
	var totalSize int64
	
	fmt.Println("\nResults:")
	fmt.Println("----------------------------------------")
	fmt.Printf("%-50s %15s %15s\n", "File", "Lines", "Size")
	fmt.Println("----------------------------------------")
	
	for _, stat := range stats {
		totalLines += stat.LineCount
		totalSize += stat.Size
		
		// Format the size for better readability
		sizeStr := formatSize(stat.Size)
		
		// Truncate long paths for better display
		path := stat.Path
		if len(path) > 50 {
			path = "..." + path[len(path)-47:]
		}
		
		fmt.Printf("%-50s %15d %15s\n", path, stat.LineCount, sizeStr)
	}
	
	fmt.Println("----------------------------------------")
	fmt.Printf("%-50s %15d %15s\n", "Total", totalLines, formatSize(totalSize))
	fmt.Println("----------------------------------------")
	
	// Print summary
	elapsed := time.Since(startTime)
	fmt.Printf("\nProcessed %d files in %v\n", len(stats), elapsed)
	fmt.Printf("Total lines: %d\n", totalLines)
	fmt.Printf("Total size: %s\n", formatSize(totalSize))
}

// countLinesInFile counts the number of lines in a file
func countLinesInFile(filePath string) (int64, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, 0, err
	}
	fileSize := fileInfo.Size()

	// Create a buffered reader
	reader := bufio.NewReaderSize(file, 1024*1024) // 1MB buffer
	
	var lineCount int64
	
	// Count lines
	for {
		_, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, 0, err
		}
		lineCount++
	}
	
	return lineCount, fileSize, nil
}

// countLinesInDirectory counts lines in all files in a directory
func countLinesInDirectory(dirPath string, recursive bool, numWorkers int, includePattern, excludePattern string) ([]FileStats, error) {
	// Collect all files to process
	var filesToProcess []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories unless recursive is enabled
		if info.IsDir() {
			if path != dirPath && !recursive {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Check include pattern
		if includePattern != "" {
			matched, err := filepath.Match(includePattern, filepath.Base(path))
			if err != nil {
				return err
			}
			if !matched {
				return nil
			}
		}
		
		// Check exclude pattern
		if excludePattern != "" {
			matched, err := filepath.Match(excludePattern, filepath.Base(path))
			if err != nil {
				return err
			}
			if matched {
				return nil
			}
		}
		
		filesToProcess = append(filesToProcess, path)
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	// Create a channel to receive results
	resultsChan := make(chan FileStats, len(filesToProcess))
	
	// Create a wait group to wait for all workers to finish
	var wg sync.WaitGroup
	
	// Create a channel to distribute work
	filesChan := make(chan string, len(filesToProcess))
	
	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range filesChan {
				lineCount, size, err := countLinesInFile(filePath)
				if err != nil {
					fmt.Printf("Error counting lines in %s: %v\n", filePath, err)
					continue
				}
				resultsChan <- FileStats{
					Path:      filePath,
					LineCount: lineCount,
					Size:      size,
				}
			}
		}()
	}
	
	// Send files to workers
	for _, filePath := range filesToProcess {
		filesChan <- filePath
	}
	close(filesChan)
	
	// Wait for all workers to finish
	wg.Wait()
	close(resultsChan)
	
	// Collect results
	var results []FileStats
	for result := range resultsChan {
		results = append(results, result)
	}
	
	return results, nil
}

// sortStats sorts the stats based on the specified criteria
func sortStats(stats []FileStats, sortBy string) {
	switch strings.ToLower(sortBy) {
	case "name":
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].Path < stats[j].Path
		})
	case "lines":
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].LineCount > stats[j].LineCount
		})
	case "size":
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].Size > stats[j].Size
		})
	}
}

// formatSize formats a file size in bytes to a human-readable format
func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
