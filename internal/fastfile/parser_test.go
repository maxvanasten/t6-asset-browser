package fastfile

import (
	"testing"

	"github.com/maxvanasten/t6-asset-browser/pkg/t6assets"
)

func TestParser_Parse(t *testing.T) {
	registry := t6assets.NewRegistry()
	parser := NewParser(registry)

	// Create test data with embedded asset names
	testData := []byte{
		0, 0, 0, 0,
		'a', 'k', '4', '7', '_', 'z', 'm', 0, // weapon
		0, 0, 0, 0,
		'v', 'i', 'e', 'w', 'm', 'o', 'd', 'e', 'l', '_', 'a', 'k', '4', '7', '_', 'm', 'p', 0, // model
		0, 0, 0, 0,
		's', 'p', 'e', 'c', 'i', 'a', 'l', 't', 'y', '_', 'a', 'r', 'm', 'o', 'r', 'v', 'e', 's', 't', 0, // perk
	}

	err := parser.Parse(testData, "test.ff")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Check weapons
	weapons := registry.GetByType(t6assets.AssetTypeWeapon)
	if len(weapons) != 1 {
		t.Errorf("expected 1 weapon, got %d", len(weapons))
	} else if weapons[0].Name != "ak47_zm" {
		t.Errorf("expected weapon name 'ak47_zm', got '%s'", weapons[0].Name)
	}

	// Check models
	models := registry.GetByType(t6assets.AssetTypeXModel)
	if len(models) != 1 {
		t.Errorf("expected 1 model, got %d", len(models))
	} else if models[0].Name != "viewmodel_ak47_mp" {
		t.Errorf("expected model name 'viewmodel_ak47_mp', got '%s'", models[0].Name)
	}

	// Check perks
	perks := registry.GetByType(t6assets.AssetTypePerk)
	if len(perks) != 1 {
		t.Errorf("expected 1 perk, got %d", len(perks))
	} else if perks[0].Name != "specialty_armorvest" {
		t.Errorf("expected perk name 'specialty_armorvest', got '%s'", perks[0].Name)
	}
}

func TestFindWeapons(t *testing.T) {
	parser := NewParser(t6assets.NewRegistry())

	testData := []byte{
		'w', 'e', 'a', 'p', 'o', 'n', '_', 'o', 'n', 'e', '_', 'z', 'm', 0,
		0, 0,
		't', '6', '_', 'w', 'p', 'n', '_', 'z', 'm', 'b', '_', 'r', 'a', 'y', 'g', 'u', 'n', '2', 0,
		0, 0,
		'n', 'o', 't', 'a', 'w', 'e', 'a', 'p', 'o', 'n', 0,
	}

	weapons := parser.findWeapons(testData)

	if len(weapons) != 2 {
		t.Errorf("expected 2 weapons, got %d", len(weapons))
	}

	// Check we found the right ones
	found := make(map[string]bool)
	for _, w := range weapons {
		found[w] = true
	}

	if !found["weapon_one_zm"] {
		t.Error("expected to find 'weapon_one_zm'")
	}
	if !found["t6_wpn_zmb_raygun2"] {
		t.Error("expected to find 't6_wpn_zmb_raygun2'")
	}
}

func TestFindXModels(t *testing.T) {
	parser := NewParser(t6assets.NewRegistry())

	testData := []byte{
		'v', 'i', 'e', 'w', 'm', 'o', 'd', 'e', 'l', '_', 't', 'e', 's', 't', 0,
		0, 0,
		'p', '_', 'b', 'o', 'd', 'y', '_', 't', 'e', 's', 't', 0,
		0, 0,
		'n', 'o', 't', 'a', 'm', 'o', 'd', 'e', 'l', 0,
	}

	models := parser.findXModels(testData)

	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}
}

func TestFindPerks(t *testing.T) {
	parser := NewParser(t6assets.NewRegistry())

	testData := []byte{
		's', 'p', 'e', 'c', 'i', 'a', 'l', 't', 'y', '_', 'a', 'r', 'm', 'o', 'r', 'v', 'e', 's', 't', 0,
		0, 0,
		's', 'p', 'e', 'c', 'i', 'a', 'l', 't', 'y', '_', 'f', 'a', 's', 't', 'r', 'e', 'l', 'o', 'a', 'd', 0,
		0, 0,
		'n', 'o', 't', 'a', 'p', 'e', 'r', 'k', 0,
	}

	perks := parser.findPerks(testData)

	if len(perks) != 2 {
		t.Errorf("expected 2 perks, got %d", len(perks))
	}
}

func TestIsValidWeaponName(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"ak47_zm", true},
		{"t6_wpn_zmb_raygun2", true},
		{"a", false},       // too short
		{"ak47 zm", false}, // contains space
		{"ak47!zm", false}, // invalid character
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidWeaponName(tt.name)
			if result != tt.expected {
				t.Errorf("isValidWeaponName(%q) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestIsValidPerkName(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"specialty_armorvest", true},
		{"specialty_fastreload", true},
		{"armorvest", false}, // missing prefix
		{"spec", false},      // too short
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidPerkName(tt.name)
			if result != tt.expected {
				t.Errorf("isValidPerkName(%q) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}
