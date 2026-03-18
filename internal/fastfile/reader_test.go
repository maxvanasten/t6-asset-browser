package fastfile

import (
	"strings"
	"testing"
)

func TestNewReader(t *testing.T) {
	r := NewReader()
	if r == nil {
		t.Fatal("NewReader() returned nil")
	}
	if len(r.key) != pcKeySize {
		t.Errorf("expected key size %d, got %d", pcKeySize, len(r.key))
	}
}

func TestVerifyMagic(t *testing.T) {
	r := NewReader()

	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "T6 unsigned magic",
			data:     append([]byte("TAff0100"), make([]byte, 32)...),
			expected: true,
		},
		{
			name:     "T6 signed magic",
			data:     append([]byte("TAffu100"), make([]byte, 32)...),
			expected: true,
		},
		{
			name:     "invalid magic",
			data:     append([]byte("INVALID!"), make([]byte, 32)...),
			expected: false,
		},
		{
			name:     "too small",
			data:     []byte("TAff"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.verifyMagic(tt.data)
			if result != tt.expected {
				t.Errorf("verifyMagic() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseHeader(t *testing.T) {
	r := NewReader()

	// Create test header data
	data := make([]byte, 40)
	copy(data[0:8], "TAff0100")
	// Version: 0x93 (PC)
	data[8] = 0x93
	data[9] = 0x00
	data[10] = 0x00
	data[11] = 0x00
	// Size: 0x1000
	data[12] = 0x00
	data[13] = 0x10
	data[14] = 0x00
	data[15] = 0x00
	// BlockCount: 0x04
	data[16] = 0x04
	data[17] = 0x00
	data[18] = 0x00
	data[19] = 0x00

	header := r.parseHeader(data)

	if string(header.Magic[:]) != "TAff0100" {
		t.Errorf("expected magic TAff0100, got %s", string(header.Magic[:]))
	}
	if header.Version != 0x93 {
		t.Errorf("expected version 0x93, got 0x%x", header.Version)
	}
	if header.Size != 0x1000 {
		t.Errorf("expected size 0x1000, got 0x%x", header.Size)
	}
	if header.BlockCount != 0x04 {
		t.Errorf("expected block count 0x04, got 0x%x", header.BlockCount)
	}
}

func TestRead(t *testing.T) {
	r := NewReader()

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid header",
			data:    append([]byte("TAff0100"), make([]byte, 32)...),
			wantErr: false,
		},
		{
			name:    "too small",
			data:    []byte("TAff"),
			wantErr: true,
			errMsg:  "file too small",
		},
		{
			name:    "invalid magic",
			data:    append([]byte("INVALID!"), make([]byte, 32)...),
			wantErr: true,
			errMsg:  "invalid FastFile magic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := r.Read(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) && err != ErrInvalidMagic {
					t.Errorf("Read() error message = %v, want containing %v", err, tt.errMsg)
				}
			}
		})
	}
}
