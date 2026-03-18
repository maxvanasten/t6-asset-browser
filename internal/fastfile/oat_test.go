package fastfile

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNewOATIntegration(t *testing.T) {
	oat := NewOATIntegration()

	// OAT might or might not be available depending on the system
	// Just verify the struct is created
	if oat == nil {
		t.Fatal("NewOATIntegration() returned nil")
	}
}

func TestFindOAT(t *testing.T) {
	path := findOAT()

	// Can't guarantee OAT is installed, so just verify function runs
	// If it finds something, verify it exists
	if path != "" {
		if _, err := exec.LookPath(path); err != nil {
			// If not in PATH, check if file exists
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("findOAT() returned path that doesn't exist: %s", path)
			}
		}
	}
}

func TestCache(t *testing.T) {
	cache, err := NewCache()
	if err != nil {
		t.Fatalf("NewCache() failed: %v", err)
	}
	defer cache.Clear()

	// Create a temporary "original" file for testing
	tempDir := t.TempDir()
	originalPath := filepath.Join(tempDir, "test.ff")
	err = os.WriteFile(originalPath, []byte("original data"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test GetCachedPath
	cachedPath := cache.GetCachedPath(originalPath)

	if !filepath.IsAbs(cachedPath) {
		t.Error("GetCachedPath() should return absolute path")
	}

	if !contains(cachedPath, "test.ff.decrypted") {
		t.Errorf("GetCachedPath() should contain 'test.ff.decrypted', got: %s", cachedPath)
	}

	// Test IsCached (should be false for non-existent cached file)
	if cache.IsCached(originalPath) {
		t.Error("IsCached() should return false for non-existent cached file")
	}

	// Test WriteCache and ReadCached
	testData := []byte("test data for caching")
	err = cache.WriteCache(originalPath, testData)
	if err != nil {
		t.Fatalf("WriteCache() failed: %v", err)
	}

	// Now it should be cached
	if !cache.IsCached(originalPath) {
		t.Error("IsCached() should return true after writing cache")
	}

	// Read it back
	readData, err := cache.ReadCached(originalPath)
	if err != nil {
		t.Fatalf("ReadCached() failed: %v", err)
	}

	if string(readData) != string(testData) {
		t.Errorf("ReadCached() returned wrong data: got %q, want %q", readData, testData)
	}

	// Test Clear
	err = cache.Clear()
	if err != nil {
		t.Fatalf("Clear() failed: %v", err)
	}

	// After clearing, cache dir should not exist
	if _, err := os.Stat(cache.cacheDir); !os.IsNotExist(err) {
		t.Error("Clear() should remove cache directory")
	}
}

func TestDecompressZoneData(t *testing.T) {
	// Test with empty data
	result, err := DecompressZoneData([]byte{})
	if err != nil {
		t.Errorf("DecompressZoneData(empty) error: %v", err)
	}
	if len(result) != 0 {
		t.Error("DecompressZoneData(empty) should return empty")
	}

	// Test with uncompressed data (should return as-is)
	uncompressed := []byte("hello world")
	result, err = DecompressZoneData(uncompressed)
	if err != nil {
		t.Errorf("DecompressZoneData(uncompressed) error: %v", err)
	}
	if string(result) != string(uncompressed) {
		t.Error("DecompressZoneData should return uncompressed data as-is")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
