package output

import (
	"fmt"
	"os"
	"path/filepath"
)

// Writer handles writing configuration output to files
type Writer struct {
	outputDir string
}

// NewWriter creates a new output writer
func NewWriter(outputDir string) *Writer {
	return &Writer{outputDir: outputDir}
}

// EnsureDir creates the output directory if it doesn't exist
func (w *Writer) EnsureDir() error {
	return os.MkdirAll(w.outputDir, 0755)
}

// Write writes the configuration to a file named after the device
func (w *Writer) Write(deviceName, group, content string) error {
	dir := filepath.Join(w.outputDir, group)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create group directory: %w", err)
	}

	filename := filepath.Join(dir, deviceName)

	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("write %s: %w", filename, err)
	}

	return nil
}

// FilePath returns the output file path for a device
func (w *Writer) FilePath(deviceName, group string) string {
	return filepath.Join(w.outputDir, group, deviceName)
}
