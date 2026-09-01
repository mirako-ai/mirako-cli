package util

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const mediaDownloadTimeout = 2 * time.Hour

var mediaDownloadClient = &http.Client{Timeout: mediaDownloadTimeout}

// ValidateMediaFile checks that a local media input is a non-empty regular
// file with an allowed extension and within the endpoint size limit.
func ValidateMediaFile(path string, maxBytes int64, allowedExtensions ...string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("input file path is required")
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to access input file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("input path must be a regular file")
	}
	if info.Size() == 0 {
		return fmt.Errorf("input file cannot be empty")
	}
	if info.Size() > maxBytes {
		return fmt.Errorf("input file is too large: %d bytes (maximum %d bytes)", info.Size(), maxBytes)
	}

	extension := strings.ToLower(filepath.Ext(path))
	for _, allowed := range allowedExtensions {
		if extension == strings.ToLower(allowed) {
			return nil
		}
	}
	return fmt.Errorf("unsupported input file type %q; supported extensions: %s", extension, strings.Join(allowedExtensions, ", "))
}

// ResolveMediaOutputPath applies the configured save directory, a timestamped
// default filename, and the required media extension.
func ResolveMediaOutputPath(requested, defaultSavePath, defaultPrefix, extension string) string {
	if requested == "" {
		now := time.Now()
		timestamp := fmt.Sprintf("%s_%03d", now.Format("20060102_150405"), now.Nanosecond()/1_000_000)
		requested = filepath.Join(defaultSavePath, fmt.Sprintf("%s_%s%s", defaultPrefix, timestamp, extension))
	}
	if !strings.HasSuffix(strings.ToLower(requested), strings.ToLower(extension)) {
		requested += extension
	}
	return requested
}

// ResolveTaskOutputPath creates a deterministic default filename for a task.
func ResolveTaskOutputPath(requested, defaultSavePath, prefix, taskID, extension string) string {
	if requested == "" {
		safeTaskID := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(taskID)
		requested = filepath.Join(defaultSavePath, fmt.Sprintf("%s_%s%s", prefix, safeTaskID, extension))
	}
	if !strings.HasSuffix(strings.ToLower(requested), strings.ToLower(extension)) {
		requested += extension
	}
	return requested
}

// DownloadMedia downloads a URL to a temporary file and atomically moves it
// into place after the complete response has been written.
func DownloadMedia(ctx context.Context, sourceURL, outputPath string) (int64, error) {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create output directory: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create download request: %w", err)
	}
	resp, err := mediaDownloadClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to download media: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return 0, fmt.Errorf("failed to download media: HTTP %d", resp.StatusCode)
	}

	tempFile, err := os.CreateTemp(dir, ".mirako-download-*.part")
	if err != nil {
		return 0, fmt.Errorf("failed to create temporary output file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	bytesWritten, copyErr := io.Copy(tempFile, resp.Body)
	closeErr := tempFile.Close()
	if copyErr != nil {
		return 0, fmt.Errorf("failed to save media: %w", copyErr)
	}
	if closeErr != nil {
		return 0, fmt.Errorf("failed to close output file: %w", closeErr)
	}
	if err := os.Chmod(tempPath, 0644); err != nil {
		return 0, fmt.Errorf("failed to set output file permissions: %w", err)
	}
	if err := replaceMediaFile(tempPath, outputPath); err != nil {
		return 0, err
	}
	return bytesWritten, nil
}

func replaceMediaFile(tempPath, outputPath string) error {
	if err := os.Rename(tempPath, outputPath); err == nil {
		return nil
	} else {
		outputInfo, statErr := os.Stat(outputPath)
		if statErr != nil || !outputInfo.Mode().IsRegular() {
			return fmt.Errorf("failed to finalize output file: %w", err)
		}

		backup, backupErr := os.CreateTemp(filepath.Dir(outputPath), ".mirako-backup-*")
		if backupErr != nil {
			return fmt.Errorf("failed to prepare output replacement: %w", backupErr)
		}
		backupPath := backup.Name()
		if closeErr := backup.Close(); closeErr != nil {
			_ = os.Remove(backupPath)
			return fmt.Errorf("failed to prepare output replacement: %w", closeErr)
		}
		if removeErr := os.Remove(backupPath); removeErr != nil {
			return fmt.Errorf("failed to prepare output replacement: %w", removeErr)
		}

		if backupErr := os.Rename(outputPath, backupPath); backupErr != nil {
			return fmt.Errorf("failed to preserve existing output file: %w", backupErr)
		}
		if retryErr := os.Rename(tempPath, outputPath); retryErr != nil {
			if restoreErr := os.Rename(backupPath, outputPath); restoreErr != nil {
				return fmt.Errorf("failed to finalize output file: %v; existing output preserved at %s after restore failed: %w", retryErr, backupPath, restoreErr)
			}
			return fmt.Errorf("failed to finalize output file: %w", retryErr)
		}
		if removeErr := os.Remove(backupPath); removeErr != nil {
			return fmt.Errorf("output replaced but failed to remove backup file %s: %w", backupPath, removeErr)
		}
		return nil
	}
}
