package fastfile

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// PC Encryption Key for T6
	pcKeySize  = 32
	blockSize  = 0x7FC0 // 32640 bytes
	headerSize = 40
)

var (
	pcKey = []byte{
		0x64, 0x1D, 0x8A, 0x2F,
		0xE3, 0x1D, 0x3A, 0xA6,
		0x36, 0x22, 0xBB, 0xC9,
		0xCE, 0x85, 0x87, 0x22,
		0x9D, 0x42, 0xB0, 0xF8,
		0xED, 0x9B, 0x92, 0x41,
		0x30, 0xBF, 0x88, 0xB6,
		0x5E, 0xDC, 0x50, 0xBE,
	}

	// Magic bytes for T6 FastFiles
	magicT6Unsigned = []byte("TAff0100")
	magicT6Signed   = []byte("TAffu100")

	ErrInvalidMagic = errors.New("invalid FastFile magic")
)

// Reader reads and decrypts BO2 FastFiles
type Reader struct {
	key []byte
}

// NewReader creates a new FastFile reader
func NewReader() *Reader {
	return &Reader{key: pcKey}
}

// XFileHeader represents the FastFile header
type XFileHeader struct {
	Magic      [8]byte
	Version    uint32
	Size       uint32
	BlockCount uint32
}

// Read reads and decrypts a FastFile, returning the decompressed zone data
func (r *Reader) Read(data []byte) ([]byte, error) {
	if len(data) < headerSize {
		return nil, fmt.Errorf("file too small: %d bytes", len(data))
	}

	if !r.verifyMagic(data) {
		return nil, ErrInvalidMagic
	}

	header := r.parseHeader(data)

	// Check if this is a valid PC FastFile (version 0x93)
	if header.Version != 0x93 {
		// Not PC version or not encrypted, return raw data after header
		return data[headerSize:], nil
	}

	// Process encrypted blocks
	decrypted, err := r.decryptBlocks(data[headerSize:], int(header.BlockCount))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return decrypted, nil
}

// decryptBlocks decrypts the block-based FastFile structure
func (r *Reader) decryptBlocks(data []byte, blockCount int) ([]byte, error) {
	var result bytes.Buffer
	offset := 0

	// The nonce is derived from the first bytes after the header
	// In BO2, the nonce is typically 8 bytes from offset 0x20 in the header
	var nonce = make([]byte, 8)
	copy(nonce, data[offset:offset+8])

	for blockNum := 0; blockNum < blockCount && offset < len(data); blockNum++ {
		// Determine block size
		currentBlockSize := blockSize
		if offset+currentBlockSize > len(data) {
			currentBlockSize = len(data) - offset
		}

		// Get encrypted block
		block := data[offset : offset+currentBlockSize]

		// Decrypt block with Salsa20
		decrypted, err := DecryptSalsa20(block, r.key, nonce)
		if err != nil {
			// If decryption fails, try using the data as-is (might not be encrypted)
			decrypted = block
		}

		// Decompress with zlib
		decompressed, err := r.Decompress(decrypted)
		if err != nil {
			// If decompression fails, use decrypted data as-is
			decompressed = decrypted
		}

		result.Write(decompressed)

		// Move to next block
		offset += currentBlockSize

		// Update nonce for next block (increment block counter)
		binary.LittleEndian.PutUint64(nonce, binary.LittleEndian.Uint64(nonce)+1)
	}

	return result.Bytes(), nil
}

func (r *Reader) verifyMagic(data []byte) bool {
	if len(data) < 8 {
		return false
	}

	magic := data[:8]

	// Check T6 magic
	if string(magic) == string(magicT6Unsigned) || string(magic) == string(magicT6Signed) {
		return true
	}

	return false
}

func (r *Reader) parseHeader(data []byte) XFileHeader {
	return XFileHeader{
		Magic:      [8]byte(data[0:8]),
		Version:    binary.LittleEndian.Uint32(data[8:12]),
		Size:       binary.LittleEndian.Uint32(data[12:16]),
		BlockCount: binary.LittleEndian.Uint32(data[16:20]),
	}
}

// Decrypt decrypts data using Salsa20 stream cipher
func (r *Reader) Decrypt(data []byte, nonce []byte) ([]byte, error) {
	return DecryptSalsa20(data, r.key, nonce)
}

// Decompress decompresses zlib-compressed data
func (r *Reader) Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return data, err // Return original if not compressed
	}
	defer reader.Close()

	return io.ReadAll(reader)
}
