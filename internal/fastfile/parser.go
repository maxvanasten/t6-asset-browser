package fastfile

import (
	"bytes"
	"encoding/binary"

	"github.com/maxvanasten/t6-asset-browser/pkg/t6assets"
)

// Parser parses zone data and extracts asset information
type Parser struct {
	registry *t6assets.Registry
}

// NewParser creates a new FastFile parser
func NewParser(registry *t6assets.Registry) *Parser {
	return &Parser{registry: registry}
}

// Parse parses zone data and extracts assets
func (p *Parser) Parse(data []byte, source string) error {
	// For MVP, we'll do a simple string scanning approach
	// This is not the most efficient but works for finding asset names

	// Look for weapon names (pattern: *_zm or t6_wpn_zmb_*)
	weapons := p.findWeapons(data)
	for _, name := range weapons {
		asset := &t6assets.Asset{
			Name:   name,
			Type:   t6assets.AssetTypeWeapon,
			Source: source,
		}
		p.registry.Add(asset)
	}

	// Look for xmodels (pattern: viewmodel_*, worldmodel_*, p_*, c_*, veh_*, zm_*)
	models := p.findXModels(data)
	for _, name := range models {
		asset := &t6assets.Asset{
			Name:   name,
			Type:   t6assets.AssetTypeXModel,
			Source: source,
		}
		p.registry.Add(asset)
	}

	// Look for perks (pattern: specialty_*)
	perks := p.findPerks(data)
	for _, name := range perks {
		asset := &t6assets.Asset{
			Name:   name,
			Type:   t6assets.AssetTypePerk,
			Source: source,
		}
		p.registry.Add(asset)
	}

	return nil
}

func (p *Parser) findWeapons(data []byte) []string {
	var weapons []string
	seen := make(map[string]bool)

	// Common weapon patterns
	patterns := [][]byte{
		[]byte("_zm"),
		[]byte("t6_wpn_zmb_"),
		[]byte("t5_wpn_zmb_"),
	}

	// Scan for null-terminated strings containing weapon patterns
	for i := 0; i < len(data)-4; i++ {
		// Check if this looks like a string (printable ASCII followed by null)
		if isPrintableASCII(data[i]) && data[i+1] == 0 {
			// Extract the string
			end := i + 1
			start := i
			for start > 0 && isPrintableASCII(data[start-1]) {
				start--
			}

			if end-start > 2 {
				str := string(data[start:end])

				// Check if it matches weapon patterns
				for _, pattern := range patterns {
					if bytes.Contains([]byte(str), pattern) && !seen[str] {
						// Additional validation: must look like a weapon name
						if isValidWeaponName(str) {
							weapons = append(weapons, str)
							seen[str] = true
						}
						break
					}
				}
			}
		}
	}

	return weapons
}

func (p *Parser) findXModels(data []byte) []string {
	var models []string
	seen := make(map[string]bool)

	// Common model patterns
	prefixes := [][]byte{
		[]byte("viewmodel_"),
		[]byte("worldmodel_"),
		[]byte("p_"),
		[]byte("c_"),
		[]byte("veh_"),
		[]byte("zm_"),
		[]byte("t6_"),
	}

	// Scan for model names
	for i := 0; i < len(data)-4; i++ {
		if isPrintableASCII(data[i]) && data[i+1] == 0 {
			end := i + 1
			start := i
			for start > 0 && isPrintableASCII(data[start-1]) {
				start--
			}

			if end-start > 3 {
				str := string(data[start:end])

				for _, prefix := range prefixes {
					if bytes.HasPrefix([]byte(str), prefix) && !seen[str] {
						if isValidModelName(str) {
							models = append(models, str)
							seen[str] = true
						}
						break
					}
				}
			}
		}
	}

	return models
}

func (p *Parser) findPerks(data []byte) []string {
	var perks []string
	seen := make(map[string]bool)

	prefix := []byte("specialty_")

	for i := 0; i < len(data)-10; i++ {
		if bytes.Equal(data[i:i+10], prefix) {
			// Found specialty_, now extract the full name
			end := i + 10
			for end < len(data) && isPrintableASCII(data[end]) {
				end++
			}

			if end > i+10 {
				str := string(data[i:end])
				if !seen[str] && isValidPerkName(str) {
					perks = append(perks, str)
					seen[str] = true
				}
			}
		}
	}

	return perks
}

func isPrintableASCII(b byte) bool {
	return b >= 32 && b <= 126
}

func isValidWeaponName(name string) bool {
	// Must be reasonable length
	if len(name) < 3 || len(name) > 64 {
		return false
	}

	// Must not contain spaces
	if bytes.Contains([]byte(name), []byte(" ")) {
		return false
	}

	// Should contain typical weapon characters
	validChars := true
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-') {
			validChars = false
			break
		}
	}

	return validChars
}

func isValidModelName(name string) bool {
	if len(name) < 3 || len(name) > 128 {
		return false
	}

	if bytes.Contains([]byte(name), []byte(" ")) {
		return false
	}

	return true
}

func isValidPerkName(name string) bool {
	// Must start with specialty_ and be reasonable length
	if len(name) < 11 || len(name) > 64 {
		return false
	}

	if !bytes.HasPrefix([]byte(name), []byte("specialty_")) {
		return false
	}

	return true
}

// ReadUint32 reads a little-endian uint32 at the given offset
func ReadUint32(data []byte, offset int) uint32 {
	return binary.LittleEndian.Uint32(data[offset : offset+4])
}

// ReadUint16 reads a little-endian uint16 at the given offset
func ReadUint16(data []byte, offset int) uint16 {
	return binary.LittleEndian.Uint16(data[offset : offset+2])
}

// FindPerks exports the perk finding functionality for testing
func (p *Parser) FindPerks(data []byte) []string {
	return p.findPerks(data)
}
