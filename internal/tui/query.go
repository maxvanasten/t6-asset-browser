package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/maxvanasten/t6-asset-browser/internal/fastfile"
	"github.com/maxvanasten/t6-asset-browser/pkg/t6assets"
)

// ExecuteQuery executes a query based on the query configuration and returns assets
func ExecuteQuery(zonePath string, query QueryConfig, useCache bool) ([]*t6assets.Asset, error) {
	// Try to load from registry cache first
	var registry *t6assets.Registry

	if useCache {
		regCache, err := fastfile.NewRegistryCacheManager()
		if err == nil {
			// Find all .ff files
			ffFiles, _ := filepath.Glob(filepath.Join(zonePath, "*.ff"))

			// Try to load cached registry
			cachedRegistry, valid := regCache.LoadRegistry(zonePath, ffFiles)
			if valid {
				fmt.Fprintf(os.Stderr, "Loaded %d assets from cache\n", len(cachedRegistry.Assets))
				registry = cachedRegistry
			}
		}
	}

	// If no valid cache, create new registry and index files
	if registry == nil {
		registry = t6assets.NewRegistry()

		if err := indexFastFilesParallel(zonePath, registry, useCache); err != nil {
			return nil, fmt.Errorf("failed to index FastFiles: %w", err)
		}

		// Save to registry cache
		if useCache {
			regCache, err := fastfile.NewRegistryCacheManager()
			if err == nil {
				ffFiles, _ := filepath.Glob(filepath.Join(zonePath, "*.ff"))
				if err := regCache.SaveRegistry(zonePath, registry, ffFiles); err == nil {
					fmt.Fprintf(os.Stderr, "Cached %d assets\n", len(registry.Assets))
				}
			}
		}
	}

	var results []*t6assets.Asset

	// Get base results based on query type
	switch query.Cmd {
	case "list":
		results = listAssets(registry, query)
	case "search":
		results = searchAssets(registry, query)
	default:
		return nil, fmt.Errorf("unsupported command: %s", query.Cmd)
	}

	return results, nil
}

// indexFastFilesParallel indexes all FastFiles in the zone directory using parallel processing
func indexFastFilesParallel(zonePath string, registry *t6assets.Registry, useCache bool) error {
	// Find all .ff files
	ffFiles, err := filepath.Glob(filepath.Join(zonePath, "*.ff"))
	if err != nil {
		return fmt.Errorf("failed to find FastFiles: %w", err)
	}

	if len(ffFiles) == 0 {
		return fmt.Errorf("no FastFiles found in %s", zonePath)
	}

	// Initialize traditional cache for raw decrypted data
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
	fmt.Fprintf(os.Stderr, "Indexing %d FastFiles...\n", totalFiles)

	// Create a worker pool for parallel processing
	// Use 4 workers or number of files, whichever is smaller
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
					fmt.Fprintf(os.Stderr, "[%d/%d] Error processing %s: %v\n", current, totalFiles, fileName, err)
				} else {
					fmt.Fprintf(os.Stderr, "[%d/%d] Indexed: %s (%d assets)\n", current, totalFiles, fileName, len(assets))
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

	// Try raw cache second
	var zoneData []byte
	if useCache && rawCache != nil && rawCache.IsCached(ffPath) {
		data, err := rawCache.ReadCached(ffPath)
		if err == nil {
			zoneData = data
		}
	}

	// Fall back to built-in decryption
	if zoneData == nil {
		data, err := os.ReadFile(ffPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		reader := fastfile.NewReader()
		decrypted, err := reader.Read(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt: %w", err)
		}
		zoneData = decrypted

		// Cache the decrypted data
		if useCache && rawCache != nil {
			rawCache.WriteCache(ffPath, zoneData)
		}
	}

	// Parse assets from zone data
	// Create a temporary registry to collect assets from this file
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

// listAssets returns filtered assets for list command
func listAssets(registry *t6assets.Registry, query QueryConfig) []*t6assets.Asset {
	var assets []*t6assets.Asset

	// Filter by map if specified
	if query.Map != "" {
		mapList := strings.Split(query.Map, ",")
		validMaps := make(map[string]bool)
		for _, m := range mapList {
			m = strings.TrimSpace(m)
			if m != "" {
				if !strings.HasSuffix(m, ".ff") {
					m = m + ".ff"
				}
				validMaps[m] = true
			}
		}

		seen := make(map[string]bool)
		for _, a := range registry.Assets {
			if validMaps[a.Source] {
				key := a.Name + "|" + a.Type.String()
				if !seen[key] {
					assets = append(assets, a)
					seen[key] = true
				}
			}
		}
	} else {
		for _, a := range registry.Assets {
			assets = append(assets, a)
		}
	}

	// Filter by type
	if query.Type != "" {
		var filtered []*t6assets.Asset
		typeList := strings.Split(query.Type, ",")
		validTypes := make(map[t6assets.AssetType]bool)
		for _, t := range typeList {
			t = strings.TrimSpace(t)
			if t != "" {
				validTypes[parseAssetType(t)] = true
			}
		}

		for _, a := range assets {
			if validTypes[a.Type] {
				filtered = append(filtered, a)
			}
		}
		assets = filtered
	}

	// Filter by pattern
	if query.Pattern != "" {
		include, exclude := parsePatterns(query.Pattern)
		var filtered []*t6assets.Asset
		for _, a := range assets {
			if matchesPatterns(a.Name, include, exclude, query.UseWildcard, query.IgnoreCase) {
				filtered = append(filtered, a)
			}
		}
		assets = filtered
	}

	return assets
}

// searchAssets returns filtered assets for search command
func searchAssets(registry *t6assets.Registry, query QueryConfig) []*t6assets.Asset {
	var results []*t6assets.Asset

	// Parse patterns once for efficiency
	include, exclude := parsePatterns(query.Pattern)

	for _, a := range registry.Assets {
		// Check pattern match
		if !matchesPatterns(a.Name, include, exclude, query.UseWildcard, query.IgnoreCase) {
			continue
		}

		// Filter by type
		if query.Type != "" {
			typeList := strings.Split(query.Type, ",")
			validTypes := make(map[t6assets.AssetType]bool)
			for _, t := range typeList {
				t = strings.TrimSpace(t)
				if t != "" {
					validTypes[parseAssetType(t)] = true
				}
			}
			if !validTypes[a.Type] {
				continue
			}
		}

		// Filter by map
		if query.Map != "" {
			mapList := strings.Split(query.Map, ",")
			validMaps := make(map[string]bool)
			for _, m := range mapList {
				m = strings.TrimSpace(m)
				if m != "" {
					if !strings.HasSuffix(m, ".ff") {
						m = m + ".ff"
					}
					validMaps[m] = true
				}
			}
			if !validMaps[a.Source] {
				continue
			}
		}

		results = append(results, a)
	}

	return results
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
