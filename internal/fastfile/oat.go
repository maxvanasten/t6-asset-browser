package fastfile

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// OATIntegration handles OpenAssetTools integration for FastFile decryption
type OATIntegration struct {
	oatPath string
}

// NewOATIntegration creates a new OAT integration
func NewOATIntegration() *OATIntegration {
	return &OATIntegration{
		oatPath: findOAT(),
	}
}

// IsAvailable checks if OpenAssetTools is installed
func (oat *OATIntegration) IsAvailable() bool {
	return oat.oatPath != ""
}

// ListAssets lists all assets in a FastFile using OAT.Unlinker
func (oat *OATIntegration) ListAssets(inputPath string) ([]string, map[string]string, error) {
	if !oat.IsAvailable() {
		return nil, nil, fmt.Errorf("OpenAssetTools not found")
	}

	// Use OAT.Unlinker with --list flag
	cmd := exec.Command(
		oat.oatPath,
		inputPath,
		"--list",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, nil, fmt.Errorf("OAT list failed: %w\nOutput: %s", err, string(output))
	}

	// Parse the output to extract asset names and types
	lines := strings.Split(string(output), "\n")
	var assetNames []string
	assetTypes := make(map[string]string)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Strip ANSI color codes
		line = stripANSI(line)

		// Parse "type, name" format
		parts := strings.SplitN(line, ",", 2)
		if len(parts) == 2 {
			assetType := strings.TrimSpace(parts[0])
			assetName := strings.TrimSpace(parts[1])

			// Only include assets we're interested in
			switch assetType {
			case "xmodel", "weapon", "material", "image", "fx":
				assetNames = append(assetNames, assetName)
				assetTypes[assetName] = assetType
			}
		}
	}

	return assetNames, assetTypes, nil
}

// stripANSI removes ANSI escape codes from string
func stripANSI(s string) string {
	// Remove common ANSI escape sequences
	// Pattern: ESC[ followed by numbers and letters, ending with m
	var result strings.Builder
	inEscape := false

	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' || s[i] == 0x1b {
			inEscape = true
			continue
		}
		if inEscape {
			if (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteByte(s[i])
	}

	return result.String()
}

// DecryptFastFile decrypts a FastFile using OpenAssetTools
func (oat *OATIntegration) DecryptFastFile(inputPath string, outputPath string) error {
	if !oat.IsAvailable() {
		return fmt.Errorf("OpenAssetTools not found")
	}

	// OAT.Unlinker dumps assets to a folder, not a single file
	// For now, return an error to fall back to built-in decryption
	return fmt.Errorf("OAT folder-based output not yet implemented, using fallback")
}

// DecryptToMemory decrypts a FastFile and returns the data in memory
func (oat *OATIntegration) DecryptToMemory(inputPath string) ([]byte, error) {
	// Create temporary file
	tempFile, err := os.CreateTemp("", "t6-asset-*.zone")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()
	defer os.Remove(tempPath)

	// Decrypt to temp file
	if err := oat.DecryptFastFile(inputPath, tempPath); err != nil {
		return nil, err
	}

	// Read the decrypted data
	return os.ReadFile(tempPath)
}

// findOAT attempts to find OpenAssetTools in common locations
func findOAT() string {
	// Common locations to check
	possiblePaths := []string{
		"OAT.Unlinker",
		"OAT.Unlinker.exe",
		"oat/OAT.Unlinker",
		"oat/OAT.Unlinker.exe",
		"/usr/local/bin/OAT.Unlinker",
		"C:/Program Files/OpenAssetTools/OAT.Unlinker.exe",
		os.Getenv("OAT_PATH"),
	}

	for _, path := range possiblePaths {
		if path == "" {
			continue
		}
		if _, err := exec.LookPath(path); err == nil {
			return path
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Try to find in PATH
	if path, err := exec.LookPath("OAT.Unlinker"); err == nil {
		return path
	}

	return ""
}

// DecryptWithFallback attempts to decrypt a FastFile using available methods
// First tries OAT, then falls back to built-in Salsa20
func DecryptWithFallback(inputPath string) ([]byte, error) {
	// Try OpenAssetTools first
	oat := NewOATIntegration()
	if oat.IsAvailable() {
		data, err := oat.DecryptToMemory(inputPath)
		if err == nil {
			return data, nil
		}
		// OAT failed, log and try fallback
		fmt.Fprintf(os.Stderr, "Warning: OAT decryption failed: %v\n", err)
	}

	// Fallback to built-in reader (may not work for encrypted files)
	reader := NewReader()
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return reader.Read(data)
}

// DecompressZoneData decompresses zlib-compressed zone data
func DecompressZoneData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	// Try to decompress
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		// Not compressed or corrupted, return as-is
		return data, nil
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		// Decompression failed, return original
		return data, nil
	}

	return decompressed, nil
}

// Cache manages decrypted FastFile caching
type Cache struct {
	cacheDir string
}

// NewCache creates a new cache instance
func NewCache() (*Cache, error) {
	cacheDir := filepath.Join(os.TempDir(), "t6-asset-browser-cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Cache{cacheDir: cacheDir}, nil
}

// GetCachedPath returns the path for a cached decrypted file
func (c *Cache) GetCachedPath(originalPath string) string {
	base := filepath.Base(originalPath)
	return filepath.Join(c.cacheDir, base+".decrypted")
}

// IsCached checks if a decrypted version exists and is up-to-date
func (c *Cache) IsCached(originalPath string) bool {
	cachedPath := c.GetCachedPath(originalPath)

	// Check if cached file exists
	cachedInfo, err := os.Stat(cachedPath)
	if err != nil {
		return false
	}

	// Check if original is newer
	originalInfo, err := os.Stat(originalPath)
	if err != nil {
		return false
	}

	// Cached file is valid if it's not older than the original
	return !cachedInfo.ModTime().Before(originalInfo.ModTime())
}

// ReadCached reads from cache if available, otherwise returns error
func (c *Cache) ReadCached(originalPath string) ([]byte, error) {
	if !c.IsCached(originalPath) {
		return nil, fmt.Errorf("file not in cache")
	}

	cachedPath := c.GetCachedPath(originalPath)
	return os.ReadFile(cachedPath)
}

// WriteCache writes decrypted data to cache
func (c *Cache) WriteCache(originalPath string, data []byte) error {
	cachedPath := c.GetCachedPath(originalPath)
	return os.WriteFile(cachedPath, data, 0644)
}

// Clear removes all cached files
func (c *Cache) Clear() error {
	return os.RemoveAll(c.cacheDir)
}
