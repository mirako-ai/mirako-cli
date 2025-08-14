package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirako-ai/mirako-cli/internal/config"
)

func TestParseAnnotationFile(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expected      []string
		expectError   bool
		errorContains string
	}{
		{
			name:     "valid annotation file",
			content:  "sample1.wav|Hello world\nsample2.mp3|How are you\nsample3.wav|This is a test",
			expected: []string{"sample1.wav", "sample2.mp3", "sample3.wav"},
		},
		{
			name:     "valid annotation file with extra whitespace",
			content:  " sample1.wav | Hello world \n  sample2.mp3  |  How are you  \n",
			expected: []string{"sample1.wav", "sample2.mp3"},
		},
		{
			name:     "valid annotation file with empty lines",
			content:  "sample1.wav|Hello world\n\nsample2.mp3|How are you\n\n",
			expected: []string{"sample1.wav", "sample2.mp3"},
		},
		{
			name:          "invalid format missing pipe",
			content:       "sample1.wav Hello world\nsample2.mp3|How are you",
			expectError:   true,
			errorContains: "invalid format on line 1",
		},
		{
			name:          "empty filename",
			content:       "|Hello world\nsample2.mp3|How are you",
			expectError:   true,
			errorContains: "empty filename on line 1",
		},
		{
			name:          "invalid file extension",
			content:       "sample1.txt|Hello world",
			expectError:   true,
			errorContains: "invalid audio file extension on line 1",
		},
		{
			name:          "empty file",
			content:       "",
			expectError:   true,
			errorContains: "no valid audio file entries found",
		},
		{
			name:          "only empty lines",
			content:       "\n\n\n",
			expectError:   true,
			errorContains: "no valid audio file entries found",
		},
		{
			name:     "mixed valid extensions",
			content:  "sample1.WAV|Hello world\nsample2.Mp3|How are you",
			expected: []string{"sample1.WAV", "sample2.Mp3"},
		},
		{
			name:     "multiple pipes in line",
			content:  "sample1.wav|Hello|world with pipe",
			expected: []string{"sample1.wav"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpFile, err := os.CreateTemp("", "annotation_*.txt")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Write content to file
			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// Test the function
			result, err := parseAnnotationFile(tmpFile.Name())

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', but got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				} else if !sliceEqual(result, tt.expected) {
					t.Errorf("Expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

func TestValidateVoiceCloneInput(t *testing.T) {
	// Create temporary directory for tests
	tmpDir, err := os.MkdirTemp("", "voice_clone_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a client for testing
	cfg := &config.Config{
		APIToken: "test-token",
		APIURL:   "https://test.example.com",
	}
	client := &Client{config: cfg}

	tests := []struct {
		name          string
		audioFiles    []string
		annotation    string
		expectError   bool
		errorContains string
	}{
		{
			name:       "valid setup",
			audioFiles: []string{"sample1.wav", "sample2.mp3", "sample3.wav"},
			annotation: "sample1.wav|Hello world\nsample2.mp3|How are you\nsample3.wav|This is a test",
		},
		{
			name:          "missing audio file",
			audioFiles:    []string{"sample1.wav", "sample2.mp3"},
			annotation:    "sample1.wav|Hello world\nsample2.mp3|How are you\nsample3.wav|This is a test",
			expectError:   true,
			errorContains: "annotation.list references",
		},
		{
			name:          "extra audio file not in annotation",
			audioFiles:    []string{"sample1.wav", "sample2.mp3", "sample3.wav", "extra.wav"},
			annotation:    "sample1.wav|Hello world\nsample2.mp3|How are you\nsample3.wav|This is a test",
			expectError:   true,
			errorContains: "found 1 audio files in directory that are not included",
		},
		{
			name:          "invalid annotation format",
			audioFiles:    []string{"sample1.wav"},
			annotation:    "sample1.wav Hello world",
			expectError:   true,
			errorContains: "invalid annotation file",
		},
		{
			name:          "empty annotation file",
			audioFiles:    []string{"sample1.wav"},
			annotation:    "",
			expectError:   true,
			errorContains: "invalid annotation file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create audio directory for this test
			audioDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(audioDir, 0755); err != nil {
				t.Fatalf("Failed to create audio dir: %v", err)
			}

			// Create audio files
			for _, filename := range tt.audioFiles {
				filePath := filepath.Join(audioDir, filename)
				if err := os.WriteFile(filePath, []byte("fake audio content"), 0644); err != nil {
					t.Fatalf("Failed to create audio file %s: %v", filename, err)
				}
			}

			// Create annotation file
			annotationPath := filepath.Join(tmpDir, tt.name+"_annotation.txt")
			if err := os.WriteFile(annotationPath, []byte(tt.annotation), 0644); err != nil {
				t.Fatalf("Failed to create annotation file: %v", err)
			}

			// Test the function
			err := client.ValidateVoiceCloneInput(audioDir, annotationPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', but got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateVoiceCloneInput_FileSystemErrors(t *testing.T) {
	cfg := &config.Config{
		APIToken: "test-token",
		APIURL:   "https://test.example.com",
	}
	client := &Client{config: cfg}

	t.Run("non-existent audio directory", func(t *testing.T) {
		// Create annotation file
		tmpFile, err := os.CreateTemp("", "annotation_*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.WriteString("sample1.wav|Hello world")
		tmpFile.Close()

		err = client.ValidateVoiceCloneInput("/non/existent/dir", tmpFile.Name())
		if err == nil {
			t.Errorf("Expected error for non-existent directory")
		}
	})

	t.Run("non-existent annotation file", func(t *testing.T) {
		// Create temporary directory
		tmpDir, err := os.MkdirTemp("", "audio_test_*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		err = client.ValidateVoiceCloneInput(tmpDir, "/non/existent/annotation.txt")
		if err == nil {
			t.Errorf("Expected error for non-existent annotation file")
		}
	})
}

func TestScanAudioFiles(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "scan_audio_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := map[string]bool{
		"sample1.wav": true,  // should be included
		"sample2.mp3": true,  // should be included
		"sample3.WAV": true,  // should be included (case insensitive)
		"sample4.MP3": true,  // should be included (case insensitive)
		"sample5.txt": false, // should be excluded
		"sample6.doc": false, // should be excluded
		"README.md":   false, // should be excluded
	}

	expectedFiles := []string{}
	for filename, shouldInclude := range testFiles {
		filePath := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
		if shouldInclude {
			expectedFiles = append(expectedFiles, filePath)
		}
	}

	// Test the function
	result, err := ScanAudioFiles(tmpDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check that we got the expected number of files
	if len(result) != len(expectedFiles) {
		t.Errorf("Expected %d files, got %d", len(expectedFiles), len(result))
	}

	// Check that all expected files are present
	resultMap := make(map[string]bool)
	for _, file := range result {
		resultMap[file] = true
	}

	for _, expectedFile := range expectedFiles {
		if !resultMap[expectedFile] {
			t.Errorf("Expected file %s not found in results", expectedFile)
		}
	}
}

func TestScanAudioFiles_EmptyDirectory(t *testing.T) {
	// Create empty temporary directory
	tmpDir, err := os.MkdirTemp("", "empty_audio_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result, err := ScanAudioFiles(tmpDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty result for empty directory, got %d files", len(result))
	}
}

func TestScanAudioFiles_NonExistentDirectory(t *testing.T) {
	_, err := ScanAudioFiles("/non/existent/directory")
	if err == nil {
		t.Errorf("Expected error for non-existent directory")
	}
}

// Helper functions
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
