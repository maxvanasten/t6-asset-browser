package fastfile

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/maxvanasten/t6-asset-browser/pkg/t6assets"
)

// AssetCacheEntry represents a cached asset with metadata
type AssetCacheEntry struct {
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Source     string    `json:"source"`
	ModifiedAt time.Time `json:"modified_at"`
	Checksum   string    `json:"checksum,omitempty"`
}

// RegistryCache stores the entire registry state
type RegistryCache struct {
	CreatedAt    time.Time           `json:"created_at"`
	ZonePath     string              `json:"zone_path"`
	Assets       []AssetCacheEntry   `json:"assets"`
	SourceFiles  []string            `json:"source_files"`
	FileVersions map[string]int64    `json:"file_versions"` // Map of file path to mod time
	TypeToFiles  map[string][]string `json:"type_to_files"` // Map of asset type to source files containing that type
}

// RegistryCacheManager manages persistent registry caching
type RegistryCacheManager struct {
	cacheDir string
	mu       sync.RWMutex
}

// NewRegistryCacheManager creates a new registry cache manager
func NewRegistryCacheManager() (*RegistryCacheManager, error) {
	cacheDir := filepath.Join(os.TempDir(), "t6-asset-browser-registry-cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create registry cache directory: %w", err)
	}

	return &RegistryCacheManager{
		cacheDir: cacheDir,
	}, nil
}

// GetCachePath returns the path to the registry cache file
func (rcm *RegistryCacheManager) GetCachePath(zonePath string) string {
	// Use a hash of the zone path as the cache filename
	return filepath.Join(rcm.cacheDir, "registry_"+sanitizePath(zonePath)+".json.gz")
}

// CachedRegistry holds a loaded registry along with type indexing info
type CachedRegistry struct {
	Registry    *t6assets.Registry
	TypeToFiles map[string][]string // Maps asset type to list of source files
}

// LoadRegistry attempts to load a cached registry
func (rcm *RegistryCacheManager) LoadRegistry(zonePath string, ffFiles []string) (*CachedRegistry, bool) {
	rcm.mu.RLock()
	defer rcm.mu.RUnlock()

	cachePath := rcm.GetCachePath(zonePath)

	// Check if cache exists
	if _, err := os.Stat(cachePath); err != nil {
		return nil, false
	}

	// Read and decompress cache
	file, err := os.Open(cachePath)
	if err != nil {
		return nil, false
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, false
	}
	defer gzReader.Close()

	var cache RegistryCache
	decoder := json.NewDecoder(gzReader)
	if err := decoder.Decode(&cache); err != nil {
		return nil, false
	}

	// Validate cache by checking if any source files have changed
	for _, ffPath := range ffFiles {
		fileInfo, err := os.Stat(ffPath)
		if err != nil {
			// Can't stat file, cache might be stale
			return nil, false
		}

		cachedModTime, exists := cache.FileVersions[ffPath]
		if !exists || fileInfo.ModTime().Unix() != cachedModTime {
			// File is new or has been modified
			return nil, false
		}
	}

	// Check if any files were removed (cache has files not in ffFiles)
	ffFilesSet := make(map[string]bool)
	for _, f := range ffFiles {
		ffFilesSet[f] = true
	}
	for cachedFile := range cache.FileVersions {
		if !ffFilesSet[cachedFile] {
			// A file was removed, cache is stale
			return nil, false
		}
	}

	// Cache is valid, restore registry
	registry := t6assets.NewRegistry()
	for _, entry := range cache.Assets {
		asset := &t6assets.Asset{
			Name:   entry.Name,
			Type:   parseAssetTypeString(entry.Type),
			Source: entry.Source,
		}
		registry.Add(asset)
	}

	return &CachedRegistry{
		Registry:    registry,
		TypeToFiles: cache.TypeToFiles,
	}, true
}

// SaveRegistry saves the registry to cache
func (rcm *RegistryCacheManager) SaveRegistry(zonePath string, registry *t6assets.Registry, ffFiles []string) error {
	rcm.mu.Lock()
	defer rcm.mu.Unlock()

	// Build file versions map
	fileVersions := make(map[string]int64)
	for _, ffPath := range ffFiles {
		fileInfo, err := os.Stat(ffPath)
		if err != nil {
			continue
		}
		fileVersions[ffPath] = fileInfo.ModTime().Unix()
	}

	// Convert registry to cache entries and build type-to-files index
	var entries []AssetCacheEntry
	seen := make(map[string]bool)
	typeToFiles := make(map[string]map[string]bool) // type -> set of files

	for _, asset := range registry.Assets {
		key := asset.Source + "/" + asset.Name
		if seen[key] {
			continue
		}
		seen[key] = true

		entries = append(entries, AssetCacheEntry{
			Name:   asset.Name,
			Type:   asset.Type.String(),
			Source: asset.Source,
		})

		// Build type to files index
		typeStr := asset.Type.String()
		if typeToFiles[typeStr] == nil {
			typeToFiles[typeStr] = make(map[string]bool)
		}
		typeToFiles[typeStr][asset.Source] = true
	}

	// Convert sets to slices
	typeToFilesSlice := make(map[string][]string)
	for assetType, filesSet := range typeToFiles {
		for file := range filesSet {
			typeToFilesSlice[assetType] = append(typeToFilesSlice[assetType], file)
		}
	}

	cache := RegistryCache{
		CreatedAt:    time.Now(),
		ZonePath:     zonePath,
		Assets:       entries,
		SourceFiles:  ffFiles,
		FileVersions: fileVersions,
		TypeToFiles:  typeToFilesSlice,
	}

	// Write compressed cache
	cachePath := rcm.GetCachePath(zonePath)
	file, err := os.Create(cachePath)
	if err != nil {
		return fmt.Errorf("failed to create cache file: %w", err)
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	encoder := json.NewEncoder(gzWriter)
	if err := encoder.Encode(cache); err != nil {
		return fmt.Errorf("failed to encode cache: %w", err)
	}

	return nil
}

// Clear removes all cached registry files
func (rcm *RegistryCacheManager) Clear() error {
	rcm.mu.Lock()
	defer rcm.mu.Unlock()

	return os.RemoveAll(rcm.cacheDir)
}

// sanitizePath creates a safe filename from a path
func sanitizePath(path string) string {
	// Replace path separators and other unsafe chars
	result := ""
	for _, c := range path {
		switch c {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			result += "_"
		default:
			result += string(c)
		}
	}
	return result
}

// parseAssetTypeString converts a string to AssetType
func parseAssetTypeString(s string) t6assets.AssetType {
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
	case "sound":
		return t6assets.AssetTypeSound
	case "fx":
		return t6assets.AssetTypeFX
	case "xanim":
		return t6assets.AssetTypeXAnim
	default:
		return t6assets.AssetTypeUnknown
	}
}
