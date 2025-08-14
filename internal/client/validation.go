// Implement helper function for pre-voice-clone validation

package client

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ScanAudioFiles scans a directory for .wav and .mp3 files
func ScanAudioFiles(dir string) ([]string, error) {
	var audioFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() &&
			(strings.ToLower(filepath.Ext(path)) == ".wav" || strings.ToLower(filepath.Ext(path)) == ".mp3") {
			audioFiles = append(audioFiles, path)
		}

		return nil
	})

	return audioFiles, err
}

// validateVoiceCloneInput validates the annotation file and audio directory before voice cloning
func (c *Client) ValidateVoiceCloneInput(audioDir, annotationFile string) error {
	// Parse annotation file
	annotatedFiles, err := parseAnnotationFile(annotationFile)
	if err != nil {
		return fmt.Errorf("invalid annotation file: %w", err)
	}

	// Get all audio files in directory
	audioFiles, err := ScanAudioFiles(audioDir)
	if err != nil {
		return fmt.Errorf("failed to scan audio directory: %w", err)
	}

	// Create map of audio files for quick lookup (using basename)
	audioFileMap := make(map[string]string)
	for _, audioFile := range audioFiles {
		basename := filepath.Base(audioFile)
		audioFileMap[basename] = audioFile
	}

	// Validate that all files in annotation.list exist in audio directory
	var missingFiles []string
	for _, annotatedFile := range annotatedFiles {
		if _, exists := audioFileMap[annotatedFile]; !exists {
			missingFiles = append(missingFiles, annotatedFile)
		}
	}

	if len(missingFiles) > 0 {
		return fmt.Errorf("annotation.list references %d audio files that don't exist in the audio directory:\n%s",
			len(missingFiles), strings.Join(missingFiles, "\n"))
	}

	// Check for audio files not included in annotation.list
	annotatedFileMap := make(map[string]bool)
	for _, annotatedFile := range annotatedFiles {
		annotatedFileMap[annotatedFile] = true
	}

	var extraFiles []string
	for basename := range audioFileMap {
		if !annotatedFileMap[basename] {
			extraFiles = append(extraFiles, basename)
		}
	}

	if len(extraFiles) > 0 {
		return fmt.Errorf("found %d audio files in directory that are not included in annotation.list:\n%s\nPlease either add them to annotation.list or remove them from the audio directory",
			len(extraFiles), strings.Join(extraFiles, "\n"))
	}

	return nil
}

// parseAnnotationFile parses the annotation file and returns a list of referenced audio files
func parseAnnotationFile(annotationFile string) ([]string, error) {
	file, err := os.Open(annotationFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open annotation file: %w", err)
	}
	defer file.Close()

	// Read the entire file content
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read annotation file: %w", err)
	}

	// Split content by lines
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	var audioFiles []string
	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue // Skip empty lines
		}

		// Split by pipe separator (common format: filename.wav|transcription)
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid format on line %d: expected 'filename|transcription', got '%s'", lineNum+1, line)
		}

		filename := strings.TrimSpace(parts[0])
		if filename == "" {
			return nil, fmt.Errorf("empty filename on line %d", lineNum+1)
		}

		// Validate file extension
		ext := strings.ToLower(filepath.Ext(filename))
		if ext != ".wav" && ext != ".mp3" {
			return nil, fmt.Errorf("invalid audio file extension on line %d: %s (only .wav and .mp3 are supported)", lineNum+1, filename)
		}

		audioFiles = append(audioFiles, filename)
	}

	if len(audioFiles) == 0 {
		return nil, fmt.Errorf("no valid audio file entries found in annotation file")
	}

	return audioFiles, nil
}
