package fastfile

import (
	"bytes"
	"testing"
)

func TestNewSalsa20State(t *testing.T) {
	key := make([]byte, 32)
	nonce := make([]byte, 8)

	// Initialize with test values
	for i := range key {
		key[i] = byte(i)
	}
	for i := range nonce {
		nonce[i] = byte(i + 100)
	}

	state := NewSalsa20State(key, nonce)

	// Verify state was created
	if state == nil {
		t.Fatal("NewSalsa20State returned nil")
	}

	// Verify constants
	if state.input[0] != 0x61707865 {
		t.Errorf("expected constant 0x61707865 at input[0], got 0x%x", state.input[0])
	}
	if state.input[5] != 0x3320646e {
		t.Errorf("expected constant 0x3320646e at input[5], got 0x%x", state.input[5])
	}
	if state.input[10] != 0x79622d32 {
		t.Errorf("expected constant 0x79622d32 at input[10], got 0x%x", state.input[10])
	}
	if state.input[15] != 0x6b206574 {
		t.Errorf("expected constant 0x6b206574 at input[15], got 0x%x", state.input[15])
	}
}

func TestQuarterRound(t *testing.T) {
	y0, y1, y2, y3 := uint32(0x00000001), uint32(0x00000000), uint32(0x00000000), uint32(0x00000000)
	z0, z1, z2, z3 := quarterRound(y0, y1, y2, y3)

	// Just verify we get some output (not all zeros or same as input)
	if z0 == y0 && z1 == y1 && z2 == y2 && z3 == y3 {
		t.Error("quarterRound didn't change values")
	}

	// Verify the function is invertible-ish
	if z0 == 0 && z1 == 0 && z2 == 0 && z3 == 0 {
		t.Error("quarterRound returned all zeros")
	}

	// Check z1 specifically matches known calculation
	// z1 = y1 ^ rotateLeft(y0+y3, 7)
	// z1 = 0 ^ rotateLeft(1+0, 7) = rotateLeft(1, 7) = 128
	if z1 != 128 {
		t.Errorf("quarterRound z1 = %d, want 128", z1)
	}
}

func TestSalsa20Block(t *testing.T) {
	// Test vector from Salsa20 spec
	key := bytes.Repeat([]byte{0x01}, 32)  // 32 bytes of 0x01
	nonce := bytes.Repeat([]byte{0x00}, 8) // 8 bytes of 0x00

	state := NewSalsa20State(key, nonce)
	block := state.salsa20Block()

	if len(block) != 64 {
		t.Errorf("expected block size 64, got %d", len(block))
	}

	// The first bytes of the keystream should not be all zeros
	allZero := true
	for _, b := range block {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("keystream block is all zeros")
	}
}

func TestDecryptSalsa20(t *testing.T) {
	key := make([]byte, 32)
	nonce := make([]byte, 8)

	// Initialize key with PC key values for realism
	copy(key, pcKey)
	for i := range nonce {
		nonce[i] = byte(i)
	}

	// Test encryption/decryption round-trip
	plaintext := []byte("Hello, World! This is a test message for Salsa20 encryption.")

	// Encrypt
	ciphertext, err := DecryptSalsa20(plaintext, key, nonce)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Decrypt (XOR with same keystream returns plaintext)
	decrypted, err := DecryptSalsa20(ciphertext, key, nonce)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("round-trip failed: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptSalsa20InvalidKey(t *testing.T) {
	_, err := DecryptSalsa20([]byte("test"), []byte("short"), make([]byte, 8))
	if err == nil {
		t.Error("expected error for invalid key size")
	}
}

func TestDecryptSalsa20InvalidNonce(t *testing.T) {
	_, err := DecryptSalsa20([]byte("test"), make([]byte, 32), []byte("short"))
	if err == nil {
		t.Error("expected error for invalid nonce size")
	}
}

func TestRotateLeft(t *testing.T) {
	tests := []struct {
		x    uint32
		bits uint
		want uint32
	}{
		{0x00000001, 1, 0x00000002},
		{0x80000000, 1, 0x00000001},
		{0x12345678, 8, 0x34567812},
	}

	for _, tt := range tests {
		got := rotateLeft(tt.x, tt.bits)
		if got != tt.want {
			t.Errorf("rotateLeft(0x%x, %d) = 0x%x, want 0x%x", tt.x, tt.bits, got, tt.want)
		}
	}
}
