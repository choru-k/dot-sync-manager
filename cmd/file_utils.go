package cmd

import (
	"io"
	"os"

	"github.com/choru-k/dot-sync-manager/internal/util"
)

// copyFile copies the file from src to dst while preserving the original permissions.
// This is a shared utility function used by multiple commands to avoid code duplication.
func copyFile(src, dst string) (err error) {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer util.CloseAndCaptureErr(sourceFile, &err)

	// Get source file info to preserve permissions
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	// Create destination file with source file's permissions
	destFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode())
	if err != nil {
		return err
	}
	defer util.CloseAndCaptureErr(destFile, &err)

	if _, err = io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}