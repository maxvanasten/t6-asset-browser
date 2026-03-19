package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/maxvanasten/t6-asset-browser/internal/fastfile"
	"github.com/maxvanasten/t6-asset-browser/pkg/t6assets"
)

// ExecuteQuery executes a query based on the query configuration and returns assets
func ExecuteQuery(zonePath string, query QueryConfig, useCache bool) ([]*t6assets.Asset, error) {
	registry := t6assets.NewRegistry()

	// Determine which files to process based on the query
	var filesToProcess []string

	if query.Map != "" {
		// Get all available .ff files first
		allFiles, _ := filepath.Glob(filepath.Join(zonePath, "*.ff"))

		// Process all files that contain any of the specified map names
		mapList := strings.Split(query.Map, ",")
		for _, ffPath := range allFiles {
			_, fileName := filepath.Split(ffPath)

			// Check if this file matches any of the requested map patterns
			for _, m := range mapList {
				m = strings.TrimSpace(m)
				if m != "" {
					// Remove .ff extension if present for matching
					searchTerm := m
					if strings.HasSuffix(searchTerm, ".ff") {
						searchTerm = searchTerm[:len(searchTerm)-3]
					}

					// Check if filename contains the map name
					if strings.Contains(fileName, searchTerm) {
						filesToProcess = append(filesToProcess, ffPath)
						break // Don't add same file multiple times
					}
				}
			}
		}

		if len(filesToProcess) > 0 {
			fmt.Fprintf(os.Stderr, "Processing %d files matching '%s'\n", len(filesToProcess), query.Map)
		}
	}

	// If no specific maps or files not found, process all files
	if len(filesToProcess) == 0 {
		filesToProcess, _ = filepath.Glob(filepath.Join(zonePath, "*.ff"))
		fmt.Fprintf(os.Stderr, "Processing all %d files\n", len(filesToProcess))
	}

	if err := indexFilesParallel(filesToProcess, registry, useCache); err != nil {
		return nil, fmt.Errorf("failed to index FastFiles: %w", err)
	}

	// Filter results based on query
	var results []*t6assets.Asset
	switch query.Cmd {
	case "list":
		results = filterAssets(registry, query)
	case "search":
		results = filterAssets(registry, query)
	default:
		return nil, fmt.Errorf("unsupported command: %s", query.Cmd)
	}

	return results, nil
}

// filterAssets filters assets based on query criteria
func filterAssets(registry *t6assets.Registry, query QueryConfig) []*t6assets.Asset {
	var results []*t6assets.Asset

	for _, asset := range registry.Assets {
		// Apply type filter
		if query.Type != "" {
			typeList := strings.Split(query.Type, ",")
			validTypes := make(map[t6assets.AssetType]bool)
			for _, t := range typeList {
				t = strings.TrimSpace(t)
				if t != "" {
					validTypes[parseAssetType(t)] = true
				}
			}
			if !validTypes[asset.Type] {
				continue
			}
		}

		// Apply map filter (check if source contains map pattern)
		if query.Map != "" {
			mapList := strings.Split(query.Map, ",")
			matched := false
			for _, m := range mapList {
				m = strings.TrimSpace(m)
				if m != "" {
					searchTerm := m
					if strings.HasSuffix(searchTerm, ".ff") {
						searchTerm = searchTerm[:len(searchTerm)-3]
					}
					if strings.Contains(asset.Source, searchTerm) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		// Apply pattern filter
		if query.Pattern != "" {
			include, exclude := parsePatterns(query.Pattern)
			if !matchesPatterns(asset.Name, include, exclude, query.UseWildcard, query.IgnoreCase) {
				continue
			}
		}

		results = append(results, asset)
	}

	return results
}

// indexFilesParallel indexes the specified files using parallel processing
func indexFilesParallel(ffFiles []string, registry *t6assets.Registry, useCache bool) error {
	if len(ffFiles) == 0 {
		return fmt.Errorf("no files to process")
	}

	// Initialize cache for raw decrypted data
	var rawCache *fastfile.Cache
	if useCache {
		rawCache, _ = fastfile.NewCache()
	}

	// Check for OAT
	oat := fastfile.NewOATIntegration()
	if oat.IsAvailable() {
		fmt.Fprintf(os.Stderr, "Using OpenAssetTools for decryption\n")
	}

	totalFiles := len(ffFiles)
	startTime := time.Now()

	// Create a worker pool
	numWorkers := 4
	if totalFiles < numWorkers {
		numWorkers = totalFiles
	}

	// Channel for files to process
	fileChan := make(chan string, totalFiles)
	for _, ffPath := range ffFiles {
		fileChan <- ffPath
	}
	close(fileChan)

	// Channel for results
	type fileResult struct {
		fileName string
		assets   []*t6assets.Asset
		err      error
	}
	resultChan := make(chan fileResult, totalFiles)

	// Progress tracking
	var processedCount int
	var mu sync.Mutex

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for ffPath := range fileChan {
				_, fileName := filepath.Split(ffPath)

				assets, err := processSingleFile(ffPath, fileName, oat, rawCache, useCache)

				mu.Lock()
				processedCount++
				current := processedCount
				mu.Unlock()

				if err != nil {
					fmt.Fprintf(os.Stderr, "[%d/%d] Error processing %s: %v\n",
						current, totalFiles, fileName, err)
				} else {
					fmt.Fprintf(os.Stderr, "[%d/%d] Indexed: %s (%d assets)\n",
						current, totalFiles, fileName, len(assets))
				}

				resultChan <- fileResult{
					fileName: fileName,
					assets:   assets,
					err:      err,
				}
			}
		}(i)
	}

	// Close result channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results and add to registry
	for result := range resultChan {
		if result.err == nil {
			for _, asset := range result.assets {
				registry.Add(asset)
			}
		}
	}

	fmt.Fprintf(os.Stderr, "Total: %d files processed, %d assets indexed in %v\n",
		totalFiles, len(registry.Assets), time.Since(startTime))

	return nil
}

// processSingleFile processes a single FastFile and returns its assets
func processSingleFile(ffPath, fileName string, oat *fastfile.OATIntegration, rawCache *fastfile.Cache, useCache bool) ([]*t6assets.Asset, error) {
	var assets []*t6assets.Asset

	// Try OAT first (fastest method)
	if oat.IsAvailable() {
		assetNames, assetTypes, err := oat.ExtractAndParseZone(ffPath)
		if err == nil && len(assetNames) > 0 {
			for _, name := range assetNames {
				assetType := parseOATAssetType(assetTypes[name])
				assets = append(assets, &t6assets.Asset{
					Name:   name,
					Type:   assetType,
					Source: fileName,
				})
			}
			return assets, nil
		}
	}

	// Fall back to built-in reader
	data, err := os.ReadFile(ffPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	reader := fastfile.NewReader()
	zoneData, err := reader.Read(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	// Cache the decrypted data
	if useCache && rawCache != nil {
		rawCache.WriteCache(ffPath, zoneData)
	}

	// Parse assets from zone data
	tempRegistry := t6assets.NewRegistry()
	parser := fastfile.NewParser(tempRegistry)

	if err := parser.Parse(zoneData, fileName); err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}

	// Extract assets from temp registry
	for _, asset := range tempRegistry.Assets {
		assets = append(assets, asset)
	}

	return assets, nil
}

// ExportToFile exports assets to a file in the specified format
// Returns the number of assets exported and any error
func ExportToFile(assets []*t6assets.Asset, format string, filename string) (int, error) {
	file, err := os.Create(filename)
	if err != nil {
		return 0, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	switch format {
	case "plain", "":
		for _, a := range assets {
			fmt.Fprintln(file, a.Name)
		}
	case "json":
		fmt.Fprintln(file, "[")
		for i, a := range assets {
			comma := ","
			if i == len(assets)-1 {
				comma = ""
			}
			fmt.Fprintf(file, "  {\"name\": \"%s\", \"type\": \"%s\", \"source\": \"%s\"}%s\n",
				a.Name, a.Type, a.Source, comma)
		}
		fmt.Fprintln(file, "]")
	case "csv":
		fmt.Fprintln(file, "name,type,source")
		for _, a := range assets {
			fmt.Fprintf(file, "%s,%s,%s\n", a.Name, a.Type, a.Source)
		}
	case "gsc":
		fmt.Fprintln(file, "array(")
		for _, a := range assets {
			fmt.Fprintf(file, "\t\"%s\",\n", a.Name)
		}
		fmt.Fprintln(file, ")")
	default:
		return 0, fmt.Errorf("unknown format: %s", format)
	}

	return len(assets), nil
}

// Helper functions

func parseAssetType(s string) t6assets.AssetType {
	switch s {
	case "weapon":
		return t6assets.AssetTypeWeapon
	case "xmodel":
		return t6assets.AssetTypeXModel
	case "perk":
		return t6assets.AssetTypePerk
	case "material":
		return t6assets.AssetTypeMaterial
	case "image":
		return t6assets.AssetTypeImage
	default:
		return t6assets.AssetTypeUnknown
	}
}

func parseOATAssetType(oatType string) t6assets.AssetType {
	switch oatType {
	case "weapon":
		return t6assets.AssetTypeWeapon
	case "xmodel":
		return t6assets.AssetTypeXModel
	case "material":
		return t6assets.AssetTypeMaterial
	case "image":
		return t6assets.AssetTypeImage
	case "fx":
		return t6assets.AssetTypeFX
	case "perk":
		return t6assets.AssetTypePerk
	default:
		return t6assets.AssetTypeUnknown
	}
}

func parsePatterns(pattern string) (include []string, exclude []string) {
	if pattern == "" {
		return nil, nil
	}

	parts := strings.Split(pattern, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "!") {
			exclude = append(exclude, part[1:])
		} else {
			include = append(include, part)
		}
	}
	return include, exclude
}

func matchesPatterns(str string, include []string, exclude []string, useWildcard bool, ignoreCase bool) bool {
	// Check all include patterns (AND logic - all must match)
	for _, pattern := range include {
		var matched bool
		if useWildcard {
			if ignoreCase {
				matched = wildcardMatch(strings.ToLower(str), strings.ToLower(pattern))
			} else {
				matched = wildcardMatch(str, pattern)
			}
		} else if ignoreCase {
			matched = containsIgnoreCase(str, pattern)
		} else {
			matched = strings.Contains(str, pattern)
		}
		if !matched {
			return false
		}
	}

	// Check all exclude patterns (AND logic - none must match)
	for _, pattern := range exclude {
		var matched bool
		if useWildcard {
			if ignoreCase {
				matched = wildcardMatch(strings.ToLower(str), strings.ToLower(pattern))
			} else {
				matched = wildcardMatch(str, pattern)
			}
		} else if ignoreCase {
			matched = containsIgnoreCase(str, pattern)
		} else {
			matched = strings.Contains(str, pattern)
		}
		if matched {
			return false
		}
	}

	return true
}

func containsIgnoreCase(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	lowerS := strings.ToLower(s)
	lowerSubstr := strings.ToLower(substr)
	return strings.Contains(lowerS, lowerSubstr)
}

func wildcardMatch(str, pattern string) bool {
	if len(pattern) == 0 {
		return len(str) == 0
	}

	if len(str) == 0 {
		for _, p := range pattern {
			if p != '*' {
				return false
			}
		}
		return true
	}

	if pattern[0] == '*' {
		return wildcardMatch(str, pattern[1:]) || wildcardMatch(str[1:], pattern)
	} else if pattern[0] == '?' || pattern[0] == str[0] {
		return wildcardMatch(str[1:], pattern[1:])
	}

	return false
}
