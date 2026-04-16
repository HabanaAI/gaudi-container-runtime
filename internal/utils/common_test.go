package utils

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestComputeChecksum(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "empty data",
			data:     []byte{},
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "simple string",
			data:     []byte("hello world"),
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name:     "binary data",
			data:     []byte{0x00, 0x01, 0x02, 0x03},
			expected: "054edec1d0211f624fed0cbca9d4f9400b0e491c43742af2c5b0abebf0c990d8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeChecksum(tt.data)
			if result != tt.expected {
				t.Errorf("ComputeChecksum() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetProcessPid(t *testing.T) {
	tests := []struct {
		name        string
		processName string
		wantErr     bool
	}{
		{
			name:        "nonexistent process",
			processName: "nonexistentprocess12345xyz",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pid, err := GetProcessPid(tt.processName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetProcessPid() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && pid == "" {
				t.Error("GetProcessPid() returned empty pid for existing process")
			}
		})
	}
}

func TestCreateFileIfNotExists(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		filePath string
		setup    func(string) error
		wantErr  bool
	}{
		{
			name:     "create new file",
			filePath: filepath.Join(tmpDir, "newfile.txt"),
			setup:    nil,
			wantErr:  false,
		},
		{
			name:     "file already exists",
			filePath: filepath.Join(tmpDir, "existing.txt"),
			setup: func(path string) error {
				return os.WriteFile(path, []byte("content"), 0644)
			},
			wantErr: false,
		},
		{
			name:     "invalid path",
			filePath: filepath.Join(tmpDir, "nonexistent", "dir", "file.txt"),
			setup:    nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				if err := tt.setup(tt.filePath); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}

			err := CreateFileIfNotExists(tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateFileIfNotExists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if _, err := os.Stat(tt.filePath); os.IsNotExist(err) {
					t.Error("CreateFileIfNotExists() file was not created")
				}
			}
		})
	}
}

type mockSerialize struct {
	data []byte
	err  error
}

func (m *mockSerialize) Serialize() ([]byte, error) {
	return m.data, m.err
}

func TestSerializeInterface(t *testing.T) {
	tests := []struct {
		name    string
		mock    Serialize
		wantErr bool
	}{
		{
			name:    "successful serialization",
			mock:    &mockSerialize{data: []byte("test data"), err: nil},
			wantErr: false,
		},
		{
			name:    "serialization error",
			mock:    &mockSerialize{data: nil, err: bytes.ErrTooLarge},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.mock.Serialize()
			if (err != nil) != tt.wantErr {
				t.Errorf("Serialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && data == nil {
				t.Error("Serialize() returned nil data")
			}
		})
	}
}
