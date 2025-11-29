package json

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// validateFilePathSecure provides enhanced security validation for file paths
func (p *Processor) validateFilePathSecure(filePath string) error {
	if filePath == "" {
		return newOperationError("validate_file_path", "file path cannot be empty", ErrOperationFailed)
	}

	// Normalize path to prevent path traversal
	cleanPath := filepath.Clean(filePath)

	// Check for path traversal patterns BEFORE converting to absolute
	// filepath.Clean normalizes ".." but we still need to detect it
	if strings.Contains(filePath, "..") {
		// After cleaning, if ".." still exists or path goes outside current dir, block it
		if strings.Contains(cleanPath, "..") || strings.HasPrefix(cleanPath, "..") {
			return newSecurityError("validate_file_path", "path traversal detected")
		}
	}

	// Check for null bytes
	if strings.Contains(cleanPath, "\x00") {
		return newSecurityError("validate_file_path", "null byte in path")
	}

	// Check path length
	if len(cleanPath) > MaxPathLength {
		return newOperationError("validate_file_path",
			fmt.Sprintf("path too long: %d > %d", len(cleanPath), MaxPathLength),
			ErrOperationFailed)
	}

	// Platform-specific security checks on clean path (before absolute conversion)
	// This ensures we catch device names in relative paths too
	if runtime.GOOS == "windows" {
		if err := validateWindowsPath(cleanPath); err != nil {
			return err
		}
	}

	// Convert to absolute path for further validation
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Platform-specific security checks on absolute path
	if runtime.GOOS != "windows" {
		if err := validateUnixPath(absPath); err != nil {
			return err
		}
	}

	// Check symlinks
	if info, err := os.Lstat(absPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			realPath, err := filepath.EvalSymlinks(absPath)
			if err != nil {
				return fmt.Errorf("cannot resolve symlink: %w", err)
			}
			// Validate the real path as well
			if runtime.GOOS != "windows" {
				if err := validateUnixPath(realPath); err != nil {
					return err
				}
			}
		}
	}

	// Check file size if exists
	if info, err := os.Stat(absPath); err == nil {
		if info.Size() > p.config.MaxJSONSize {
			return newSizeLimitError("validate_file_path", info.Size(), p.config.MaxJSONSize)
		}
	}

	return nil
}

// validateUnixPath validates Unix-specific path security
func validateUnixPath(absPath string) error {
	lowerPath := strings.ToLower(absPath)

	// Block access to critical system directories
	criticalDirs := []string{
		"/dev/",
		"/proc/",
		"/sys/",
		"/etc/passwd",
		"/etc/shadow",
		"/etc/sudoers",
		"/root/",
	}

	for _, dir := range criticalDirs {
		if strings.HasPrefix(lowerPath, dir) {
			return newSecurityError("validate_unix_path", "access to system directory not allowed")
		}
	}

	return nil
}

// validateWindowsPath validates Windows-specific path security
func validateWindowsPath(absPath string) error {
	// Check for UNC paths
	if strings.HasPrefix(absPath, "\\\\") {
		if strings.Contains(absPath, "\x00") {
			return newSecurityError("validate_windows_path", "invalid UNC path")
		}
	}

	// Check reserved device names
	filename := strings.ToUpper(filepath.Base(absPath))
	if idx := strings.LastIndex(filename, "."); idx > 0 {
		filename = filename[:idx]
	}

	reserved := []string{"CON", "PRN", "AUX", "NUL"}
	for _, name := range reserved {
		if filename == name {
			return newSecurityError("validate_windows_path", "Windows reserved device name")
		}
	}

	// Check COM1-9 and LPT1-9
	if len(filename) == 4 && filename[3] >= '1' && filename[3] <= '9' {
		prefix := filename[:3]
		if prefix == "COM" || prefix == "LPT" {
			return newSecurityError("validate_windows_path", "Windows reserved device name")
		}
	}

	return nil
}
