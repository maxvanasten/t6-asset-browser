package fastfile

import (
	"encoding/binary"
	"fmt"
)

// Salsa20 implementation for BO2 FastFile decryption
// Based on the Salsa20/12 stream cipher specification

type salsa20State struct {
	input [16]uint32
}

// NewSalsa20State creates a new Salsa20 state with the given key and nonce
func NewSalsa20State(key []byte, nonce []byte) *salsa20State {
	if len(key) != 32 {
		panic("key must be 32 bytes")
	}
	if len(nonce) != 8 {
		panic("nonce must be 8 bytes")
	}

	s := &salsa20State{}

	// Constants for Salsa20
	s.input[0] = 0x61707865  // "expa"
	s.input[5] = 0x3320646e  // "nd 3"
	s.input[10] = 0x79622d32 // "2-by"
	s.input[15] = 0x6b206574 // "te k"

	// Key
	s.input[1] = binary.LittleEndian.Uint32(key[0:4])
	s.input[2] = binary.LittleEndian.Uint32(key[4:8])
	s.input[3] = binary.LittleEndian.Uint32(key[8:12])
	s.input[4] = binary.LittleEndian.Uint32(key[12:16])
	s.input[11] = binary.LittleEndian.Uint32(key[16:20])
	s.input[12] = binary.LittleEndian.Uint32(key[20:24])
	s.input[13] = binary.LittleEndian.Uint32(key[24:28])
	s.input[14] = binary.LittleEndian.Uint32(key[28:32])

	// Nonce (block counter)
	s.input[6] = binary.LittleEndian.Uint32(nonce[0:4])
	s.input[7] = binary.LittleEndian.Uint32(nonce[4:8])
	s.input[8] = 0 // Block counter low
	s.input[9] = 0 // Block counter high

	return s
}

// rotateLeft rotates a uint32 left by bits
func rotateLeft(x uint32, bits uint) uint32 {
	return (x << bits) | (x >> (32 - bits))
}

// quarterRound performs the Salsa20 quarter round
func quarterRound(y0, y1, y2, y3 uint32) (uint32, uint32, uint32, uint32) {
	z1 := y1 ^ rotateLeft(y0+y3, 7)
	z2 := y2 ^ rotateLeft(z1+y0, 9)
	z3 := y3 ^ rotateLeft(z2+z1, 13)
	z0 := y0 ^ rotateLeft(z3+z2, 18)
	return z0, z1, z2, z3
}

// salsa20Block generates a 64-byte keystream block
func (s *salsa20State) salsa20Block() []byte {
	// Copy the input state
	x := s.input

	// 10 double rounds (20 rounds total)
	for i := 0; i < 10; i++ {
		// Column rounds
		x[4], x[0], x[12], x[8] = quarterRound(x[0], x[4], x[8], x[12])
		x[9], x[5], x[1], x[13] = quarterRound(x[5], x[9], x[13], x[1])
		x[14], x[10], x[6], x[2] = quarterRound(x[10], x[14], x[2], x[6])
		x[3], x[15], x[11], x[7] = quarterRound(x[15], x[3], x[7], x[11])

		// Row rounds
		x[1], x[2], x[3], x[0] = quarterRound(x[0], x[1], x[2], x[3])
		x[6], x[7], x[4], x[5] = quarterRound(x[5], x[6], x[7], x[4])
		x[11], x[8], x[9], x[10] = quarterRound(x[10], x[11], x[8], x[9])
		x[12], x[13], x[14], x[15] = quarterRound(x[15], x[12], x[13], x[14])
	}

	// Add initial state to working state
	for i := 0; i < 16; i++ {
		x[i] += s.input[i]
	}

	// Serialize to bytes (little-endian)
	output := make([]byte, 64)
	for i := 0; i < 16; i++ {
		binary.LittleEndian.PutUint32(output[i*4:(i+1)*4], x[i])
	}

	// Increment block counter
	s.input[8]++
	if s.input[8] == 0 {
		s.input[9]++
	}

	return output
}

// Decrypt decrypts data using Salsa20 with the given key and nonce
func DecryptSalsa20(data []byte, key []byte, nonce []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}
	if len(nonce) != 8 {
		return nil, fmt.Errorf("nonce must be 8 bytes, got %d", len(nonce))
	}

	state := NewSalsa20State(key, nonce)
	output := make([]byte, len(data))

	// Process in 64-byte blocks
	for i := 0; i < len(data); i += 64 {
		block := state.salsa20Block()
		end := i + 64
		if end > len(data) {
			end = len(data)
		}

		// XOR with keystream
		for j := i; j < end; j++ {
			output[j] = data[j] ^ block[j-i]
		}
	}

	return output, nil
}
