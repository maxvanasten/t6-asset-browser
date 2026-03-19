package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/maxvanasten/t6-asset-browser/internal/fastfile"
	"github.com/maxvanasten/t6-asset-browser/pkg/t6assets"
)

const version = "0.5.0"

func main() {
	startTime := time.Now()

	var (
		zoneDir     = flag.String("zone-dir", "", "Path to zone directory (default: auto-detect)")
		command     = flag.String("cmd", "index", "Command: index, list, search, export")
		assetMap    = flag.String("map", "", "Map name(s) (e.g., zm_tomb or zm_tomb,zm_prison)")
		assetType   = flag.String("type", "", "Asset type(s): weapon, xmodel, perk, material, image (comma-separated for multiple)")
		format      = flag.String("format", "plain", "Export format: plain, json, csv, gsc")
		output      = flag.String("output", "", "Output file (default: stdout)")
		useCache    = flag.Bool("cache", true, "Use caching for decrypted files")
		clearCache  = flag.Bool("clear-cache", false, "Clear cache before running")
		ignoreCase  = flag.Bool("i", false, "Case-insensitive search")
		showVersion = flag.Bool("version", false, "Show version and exit")
		sortBy      = flag.String("sort", "name", "Sort output by: name, type, source")
		useWildcard = flag.Bool("wildcard", false, "Use wildcard pattern matching (* and ?)")
		pattern     = flag.String("pattern", "", "Search pattern (required for search command)")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("t6-asset-browser version %s\n", version)
		os.Exit(0)
	}

	if *clearCache {
		cache, err := fastfile.NewCache()
		if err == nil {
			cache.Clear()
			fmt.Fprintln(os.Stderr, "Cache cleared")
		}
		if *command == "" {
			return
		}
	}

	// Auto-detect zone directory if not specified
	zonePath := *zoneDir
	if zonePath == "" {
		zonePath = detectZoneDir()
	}

	if zonePath == "" {
		fmt.Fprintf(os.Stderr, "Error: Could not detect zone directory. Use -zone-dir flag.\n")
		fmt.Fprintf(os.Stderr, "\nCommon locations:\n")
		fmt.Fprintf(os.Stderr, "  Steam: ~/.steam/steam/steamapps/common/Call of Duty Black Ops II/zone/all\n")
		fmt.Fprintf(os.Stderr, "  Plutonium: %%LOCALAPPDATA%%/Plutonium/storage/t6/zone\n")
		os.Exit(1)
	}

	// Create registry
	registry := t6assets.NewRegistry()

	// Execute command
	switch *command {
	case "index":
		err := indexFastFiles(zonePath, registry, *useCache)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error indexing: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Indexed %d assets (took %v)\n", len(registry.Assets), time.Since(startTime))

	case "list":
		err := indexFastFiles(zonePath, registry, *useCache)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		listAssets(registry, *assetMap, *assetType, *pattern, *sortBy, *ignoreCase, *useWildcard)
		fmt.Fprintf(os.Stderr, "\nTime: %v\n", time.Since(startTime))

	case "search":
		if *pattern == "" {
			fmt.Fprintf(os.Stderr, "Error: search requires -pattern flag\n")
			os.Exit(1)
		}
		err := indexFastFiles(zonePath, registry, *useCache)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		searchAssets(registry, *pattern, *assetType, *assetMap, *ignoreCase, *sortBy, *useWildcard)
		fmt.Fprintf(os.Stderr, "\nTime: %v\n", time.Since(startTime))

	case "export":
		err := indexFastFiles(zonePath, registry, *useCache)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		err = exportAssets(registry, *assetMap, *assetType, *pattern, *format, *output, *sortBy, *ignoreCase, *useWildcard)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "\nTime: %v\n", time.Since(startTime))

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", *command)
		fmt.Fprintf(os.Stderr, "\nAvailable commands:\n")
		fmt.Fprintf(os.Stderr, "  index  - Index all FastFiles\n")
		fmt.Fprintf(os.Stderr, "  list   - List assets (use -map and -type flags)\n")
		fmt.Fprintf(os.Stderr, "  search - Search for assets by pattern\n")
		fmt.Fprintf(os.Stderr, "  export - Export assets to file\n")
		os.Exit(1)
	}
}

func detectZoneDir() string {
	// Try common locations
	locations := []string{
		os.Getenv("T6_ZONE_DIR"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Plutonium", "storage", "t6", "zone"),
		filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local", "Plutonium", "storage", "t6", "zone"),
		// Steam paths
		filepath.Join(os.Getenv("HOME"), ".steam", "steam", "steamapps", "common", "Call of Duty Black Ops II", "zone", "all"),
		"/home/" + os.Getenv("USER") + "/.steam/steam/steamapps/common/Call of Duty Black Ops II/zone/all",
	}

	for _, loc := range locations {
		if loc != "" {
			if _, err := os.Stat(loc); err == nil {
				return loc
			}
		}
	}

	return ""
}

func indexFastFiles(zonePath string, registry *t6assets.Registry, useCache bool) error {
	// Find all .ff files
	ffFiles, err := filepath.Glob(filepath.Join(zonePath, "*.ff"))
	if err != nil {
		return fmt.Errorf("failed to find FastFiles: %w", err)
	}

	if len(ffFiles) == 0 {
		return fmt.Errorf("no FastFiles found in %s", zonePath)
	}

	totalFiles := len(ffFiles)

	// Initialize cache if requested
	var cache *fastfile.Cache
	if useCache {
		cache, _ = fastfile.NewCache()
	}

	// Check for OAT
	oat := fastfile.NewOATIntegration()
	if oat.IsAvailable() {
		fmt.Fprintf(os.Stderr, "Using OpenAssetTools for decryption\n")
	} else {
		fmt.Fprintf(os.Stderr, "Warning: OpenAssetTools not found. Trying built-in decryption...\n")
		fmt.Fprintf(os.Stderr, "For best results, install OpenAssetTools from https://github.com/Laupetin/OpenAssetTools\n")
	}

	parser := fastfile.NewParser(registry)
	processed := 0

	for i, ffPath := range ffFiles {
		_, fileName := filepath.Split(ffPath)

		var zoneData []byte

		// Try cache first
		if useCache && cache != nil && cache.IsCached(ffPath) {
			zoneData, err = cache.ReadCached(ffPath)
			if err == nil {
				fmt.Fprintf(os.Stderr, "[%d/%d] [cached] Indexed: %s\n", i+1, totalFiles, fileName)
				processed++
				continue
			}
		}

		// If not in cache or read failed, decrypt
		if zoneData == nil {
			if oat.IsAvailable() {
				// Use OAT ExtractAndParseZone for complete asset list
				assetNames, assetTypes, oatErr := oat.ExtractAndParseZone(ffPath)
				if oatErr == nil && len(assetNames) > 0 {
					// Successfully got asset names from OAT zone file
					for _, name := range assetNames {
						assetType := parseOATAssetType(assetTypes[name])
						asset := &t6assets.Asset{
							Name:   name,
							Type:   assetType,
							Source: fileName,
						}
						registry.Add(asset)
					}

					// OAT extraction succeeded - assets already added to registry
					// Skip raw file parsing to avoid corrupted data from Salsa20

					processed++
					fmt.Fprintf(os.Stderr, "[%d/%d] Indexed: %s\n", i+1, totalFiles, fileName)
					continue
				}
				// OAT extraction failed, fall back to old method
				fmt.Fprintf(os.Stderr, "Warning: OAT extraction failed for %s: %v, trying fallback\n", fileName, oatErr)

				// Fallback to list mode
				assetNames, assetTypes, listErr := oat.ListAssets(ffPath)
				if listErr == nil && len(assetNames) > 0 {
					for _, name := range assetNames {
						assetType := parseOATAssetType(assetTypes[name])
						asset := &t6assets.Asset{
							Name:   name,
							Type:   assetType,
							Source: fileName,
						}
						registry.Add(asset)
					}
					processed++
					fmt.Fprintf(os.Stderr, "[%d/%d] Indexed: %s\n", i+1, totalFiles, fileName)
					continue
				}
			}

			// Fall back to built-in reader
			data, readErr := os.ReadFile(ffPath)
			if readErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to read %s: %v\n", fileName, readErr)
				continue
			}
			reader := fastfile.NewReader()
			zoneData, err = reader.Read(data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", fileName, err)
				continue
			}

			// Cache the decrypted data
			if useCache && cache != nil && zoneData != nil {
				cache.WriteCache(ffPath, zoneData)
			}
		}

		// Parse assets
		err = parser.Parse(zoneData, fileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to extract from %s: %v\n", fileName, err)
			continue
		}

		processed++
		fmt.Fprintf(os.Stderr, "[%d/%d] Indexed: %s\n", i+1, totalFiles, fileName)
	}

	if processed == 0 {
		return fmt.Errorf("no FastFiles could be processed")
	}

	return nil
}

func listAssets(registry *t6assets.Registry, sourceMap, assetType, pattern string, sortBy string, ignoreCase bool, useWildcard bool) {
	var assets []*t6assets.Asset

	if sourceMap != "" {
		// Support comma-separated list of maps
		mapList := strings.Split(sourceMap, ",")
		validMaps := make(map[string]bool)
		for _, m := range mapList {
			m = strings.TrimSpace(m)
			if m != "" {
				// Support both with and without .ff extension
				if !strings.HasSuffix(m, ".ff") {
					m = m + ".ff"
				}
				validMaps[m] = true
			}
		}

		// Get assets from all specified maps
		seen := make(map[string]bool)
		for _, a := range registry.Assets {
			if validMaps[a.Source] {
				// Deduplicate by name+type
				key := a.Name + "|" + a.Type.String()
				if !seen[key] {
					assets = append(assets, a)
					seen[key] = true
				}
			}
		}
	} else {
		// Get all
		for _, a := range registry.Assets {
			assets = append(assets, a)
		}
	}

	// Filter by type if specified (supports comma-separated list)
	if assetType != "" {
		var filtered []*t6assets.Asset
		// Parse comma-separated types
		typeList := strings.Split(assetType, ",")
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

	// Filter by pattern if specified
	if pattern != "" {
		var filtered []*t6assets.Asset
		for _, a := range assets {
			var matched bool
			if useWildcard {
				if ignoreCase {
					matched = wildcardMatch(strings.ToLower(a.Name), strings.ToLower(pattern))
				} else {
					matched = wildcardMatch(a.Name, pattern)
				}
			} else if ignoreCase {
				matched = containsIgnoreCase(a.Name, pattern)
			} else {
				matched = strings.Contains(a.Name, pattern)
			}
			if matched {
				filtered = append(filtered, a)
			}
		}
		assets = filtered
	}

	// Sort assets
	sortAssets(assets, sortBy)

	// Print
	for _, a := range assets {
		fmt.Printf("[%s] %s (from %s)\n", a.Type, a.Name, a.Source)
	}

	fmt.Printf("\nTotal: %d assets\n", len(assets))
}

func searchAssets(registry *t6assets.Registry, pattern, assetType, sourceMap string, ignoreCase bool, sortBy string, useWildcard bool) {
	var results []*t6assets.Asset

	for _, a := range registry.Assets {
		// Check pattern match
		var matched bool
		if useWildcard {
			if ignoreCase {
				matched = wildcardMatch(strings.ToLower(a.Name), strings.ToLower(pattern))
			} else {
				matched = wildcardMatch(a.Name, pattern)
			}
		} else if ignoreCase {
			matched = containsIgnoreCase(a.Name, pattern)
		} else {
			matched = strings.Contains(a.Name, pattern)
		}
		if !matched {
			continue
		}

		// Filter by type (supports comma-separated list)
		if assetType != "" {
			typeList := strings.Split(assetType, ",")
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

		// Filter by map (supports comma-separated list)
		if sourceMap != "" {
			mapList := strings.Split(sourceMap, ",")
			validMaps := make(map[string]bool)
			for _, m := range mapList {
				m = strings.TrimSpace(m)
				if m != "" {
					// Support both with and without .ff extension
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

	// Sort results
	sortAssets(results, sortBy)

	for _, a := range results {
		fmt.Printf("[%s] %s (from %s)\n", a.Type, a.Name, a.Source)
	}

	fmt.Printf("\nFound: %d matches\n", len(results))
}

func exportAssets(registry *t6assets.Registry, sourceMap, assetType, pattern string, format, output, sortBy string, ignoreCase bool, useWildcard bool) error {
	// Get assets to export
	var assets []*t6assets.Asset

	if sourceMap != "" {
		// Support comma-separated list of maps
		mapList := strings.Split(sourceMap, ",")
		validMaps := make(map[string]bool)
		for _, m := range mapList {
			m = strings.TrimSpace(m)
			if m != "" {
				// Support both with and without .ff extension
				if !strings.HasSuffix(m, ".ff") {
					m = m + ".ff"
				}
				validMaps[m] = true
			}
		}

		// Get assets from all specified maps
		seen := make(map[string]bool)
		for _, a := range registry.Assets {
			if validMaps[a.Source] {
				// Deduplicate by name+type
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

	// Filter by type (supports comma-separated list)
	if assetType != "" {
		var filtered []*t6assets.Asset
		typeList := strings.Split(assetType, ",")
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

	// Filter by pattern if specified
	if pattern != "" {
		var filtered []*t6assets.Asset
		for _, a := range assets {
			var matched bool
			if useWildcard {
				if ignoreCase {
					matched = wildcardMatch(strings.ToLower(a.Name), strings.ToLower(pattern))
				} else {
					matched = wildcardMatch(a.Name, pattern)
				}
			} else if ignoreCase {
				matched = containsIgnoreCase(a.Name, pattern)
			} else {
				matched = strings.Contains(a.Name, pattern)
			}
			if matched {
				filtered = append(filtered, a)
			}
		}
		assets = filtered
	}

	// Sort assets
	sortAssets(assets, sortBy)

	// Determine output
	out := os.Stdout
	if output != "" {
		// Validate output path
		dir := filepath.Dir(output)
		if dir != "" && dir != "." {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return fmt.Errorf("output directory does not exist: %s", dir)
			}
		}

		f, err := os.Create(output)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	// Export based on format
	switch format {
	case "plain":
		for _, a := range assets {
			fmt.Fprintln(out, a.Name)
		}
	case "json":
		// Simple JSON output
		fmt.Fprintln(out, "[")
		for i, a := range assets {
			comma := ","
			if i == len(assets)-1 {
				comma = ""
			}
			fmt.Fprintf(out, "  {\"name\": \"%s\", \"type\": \"%s\", \"source\": \"%s\"}%s\n",
				a.Name, a.Type, a.Source, comma)
		}
		fmt.Fprintln(out, "]")
	case "csv":
		fmt.Fprintln(out, "name,type,source")
		for _, a := range assets {
			fmt.Fprintf(out, "%s,%s,%s\n", a.Name, a.Type, a.Source)
		}
	case "gsc":
		// Generate GSC array only
		fmt.Fprintln(out, "array(")
		for _, a := range assets {
			fmt.Fprintf(out, "\t\"%s\",\n", a.Name)
		}
		fmt.Fprintln(out, ")")
	default:
		return fmt.Errorf("unknown format: %s", format)
	}

	return nil
}

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

// parseOATAssetType converts OAT asset type names to our AssetType
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

func containsIgnoreCase(s, substr string) bool {
	// Simple case-insensitive contains
	if len(substr) > len(s) {
		return false
	}
	// TODO: implement proper case-insensitive search
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// sortAssets sorts assets by the specified criteria
func sortAssets(assets []*t6assets.Asset, sortBy string) {
	switch sortBy {
	case "name":
		// Sort by name (already default, but make it explicit)
		for i := 0; i < len(assets); i++ {
			for j := i + 1; j < len(assets); j++ {
				if assets[i].Name > assets[j].Name {
					assets[i], assets[j] = assets[j], assets[i]
				}
			}
		}
	case "type":
		// Sort by type, then by name
		for i := 0; i < len(assets); i++ {
			for j := i + 1; j < len(assets); j++ {
				if assets[i].Type > assets[j].Type ||
					(assets[i].Type == assets[j].Type && assets[i].Name > assets[j].Name) {
					assets[i], assets[j] = assets[j], assets[i]
				}
			}
		}
	case "source":
		// Sort by source, then by name
		for i := 0; i < len(assets); i++ {
			for j := i + 1; j < len(assets); j++ {
				if assets[i].Source > assets[j].Source ||
					(assets[i].Source == assets[j].Source && assets[i].Name > assets[j].Name) {
					assets[i], assets[j] = assets[j], assets[i]
				}
			}
		}
	}
	// If sortBy is not recognized, leave unsorted (defaults to name order from index)
}

// wildcardMatch matches a string against a pattern with * and ? wildcards
// * matches any sequence of characters, ? matches any single character
func wildcardMatch(str, pattern string) bool {
	// Simple recursive implementation
	if len(pattern) == 0 {
		return len(str) == 0
	}

	if len(str) == 0 {
		// Pattern can match empty string if it's all *
		for _, p := range pattern {
			if p != '*' {
				return false
			}
		}
		return true
	}

	// Check first character
	if pattern[0] == '*' {
		// * matches any sequence: either skip the * or consume a character
		return wildcardMatch(str, pattern[1:]) || wildcardMatch(str[1:], pattern)
	} else if pattern[0] == '?' || pattern[0] == str[0] {
		// ? matches any single character, or exact match
		return wildcardMatch(str[1:], pattern[1:])
	}

	return false
}

// createZoneDataFromAssetNames creates synthetic zone data from OAT asset names
// This allows the parser to work with OAT's list output
func createZoneDataFromAssetNames(assetNames []string) []byte {
	var buf bytes.Buffer

	for _, name := range assetNames {
		// Write asset name followed by null terminator
		buf.WriteString(name)
		buf.WriteByte(0)
	}

	return buf.Bytes()
}
