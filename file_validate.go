package json

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// validateFilePath provides enhanced security validation for file paths
func (p *Processor) validateFilePath(filePath string) error {
	if filePath == "" {
		return newOperationError("validate_file_path", "file path cannot be empty", ErrOperationFailed)
	}

	// SECURITY: Check for null bytes before any processing
	if strings.Contains(filePath, "\x00") {
		return newSecurityError("validate_file_path", "null byte in path")
	}

	// SECURITY: Check for path traversal patterns BEFORE normalization
	if containsPathTraversal(filePath) {
		return newSecurityError("validate_file_path", "path traversal pattern detected")
	}

	// Platform-specific security checks on original path (before normalization)
	if runtime.GOOS == "windows" {
		if err := validateWindowsPath(filePath); err != nil {
			return err
		}
	}

	// Normalize the path after security checks
	cleanPath := filepath.Clean(filePath)

	// Check path length after cleaning
	if len(cleanPath) > MaxPathLength {
		return newOperationError("validate_file_path",
			fmt.Sprintf("path too long: %d > %d", len(cleanPath), MaxPathLength),
			ErrOperationFailed)
	}

	// Convert to absolute path for further validation
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return newOperationError("validate_file_path", "invalid path", err)
	}

	// Platform-specific security checks on absolute path
	if runtime.GOOS != "windows" {
		if err := validateUnixPath(absPath); err != nil {
			return err
		}
	}

	// Check symlinks with loop detection
	if info, err := os.Lstat(absPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			realPath, err := filepath.EvalSymlinks(absPath)
			if err != nil {
				return newOperationError("validate_file_path", "cannot resolve symlink", err)
			}

			// Ensure symlink doesn't escape to restricted areas
			if runtime.GOOS != "windows" {
				if err := validateUnixPath(realPath); err != nil {
					return err
				}
			} else {
				if err := validateWindowsPath(realPath); err != nil {
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

// containsPathTraversal checks for path traversal patterns comprehensively
func containsPathTraversal(path string) bool {
	// Check for various path traversal patterns including bypass attempts
	patterns := []string{
		"..",         // Standard traversal
		"%2e%2e",     // URL encoded
		"%252e%252e", // Double URL encoded
		"..%2f",      // Mixed encoding
		"..%5c",      // Windows backslash encoded
		"..%c0%af",   // UTF-8 overlong encoding
		"..%c1%9c",   // UTF-8 overlong encoding variant
		".%2e",       // Partial encoding
		"%2e.",       // Partial encoding variant
		"%2e%2e%2f",  // Full URL encoded traversal
		"%2e%2e%5c",  // Windows variant
		"..\\",       // Windows backslash
		"..\\/",      // Mixed separators
		"..%00",      // Null byte injection
		"..%0a",      // Newline injection
		"..%0d",      // Carriage return injection
		"..%09",      // Tab injection
		"..%20",      // Space injection
		"....//",     // Double dot with double slash
		"....\\\\",   // Double dot with double backslash
		".....",      // Five consecutive dots
		"......",     // Six consecutive dots (defense in depth)
	}

	lowerPath := strings.ToLower(path)
	for _, pattern := range patterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	// Additional check: consecutive dots in any form (3 or more)
	if containsConsecutiveDots(path, 3) {
		return true
	}

	// Check for encoded null bytes and control characters
	encodedNulls := []string{"%00", "%0a", "%0d", "%09", "%20"}
	for _, encoded := range encodedNulls {
		if strings.Contains(lowerPath, encoded) {
			return true
		}
	}

	// Check for partial double encoding bypass attempts
	if containsPartialDoubleEncoding(lowerPath) {
		return true
	}

	return false
}

// containsConsecutiveDots checks for consecutive dots in any form
func containsConsecutiveDots(path string, minCount int) bool {
	dotCount := 0
	for _, r := range path {
		if r == '.' {
			dotCount++
			if dotCount >= minCount {
				return true
			}
		} else {
			dotCount = 0
		}
	}
	return false
}

// containsPartialDoubleEncoding checks for partial double encoding bypass attempts
func containsPartialDoubleEncoding(path string) bool {
	patterns := []string{"%2e%2", "%25%2e", "%2f%2", "%5c%2"}
	for _, pattern := range patterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
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
		"/etc/hosts",
		"/etc/fstab",
		"/etc/crontab",
		"/root/",
		"/boot/",
		"/var/log/",
		"/usr/bin/",
		"/usr/sbin/",
		"/sbin/",
		"/bin/",
	}

	for _, dir := range criticalDirs {
		if strings.HasPrefix(lowerPath, dir) {
			return newSecurityError("validate_unix_path", "access to system directory not allowed")
		}
	}

	// Additional security checks for Unix systems
	if strings.Contains(absPath, "/..") || strings.Contains(absPath, "../") {
		return newSecurityError("validate_unix_path", "path traversal detected")
	}

	return nil
}

// validateWindowsPath validates Windows-specific path security
func validateWindowsPath(absPath string) error {
	// Check for UNC paths
	if strings.HasPrefix(absPath, "\\\\") || strings.HasPrefix(absPath, "//") {
		return newSecurityError("validate_windows_path", "UNC paths not allowed")
	}

	// Extract filename for device name checking
	filename := strings.ToUpper(filepath.Base(absPath))
	if idx := strings.LastIndex(filename, "."); idx > 0 {
		filename = filename[:idx]
	}

	// Check reserved device names (complete list)
	reserved := []string{"CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$"}
	for _, name := range reserved {
		if filename == name {
			return newSecurityError("validate_windows_path", "Windows reserved device name")
		}
	}

	// Additional check for alternate data streams (ADS)
	if strings.Contains(absPath, ":") {
		parts := strings.SplitN(absPath, ":", 2)
		if len(parts) == 2 {
			// Check if it looks like a drive letter pattern
			if len(parts[0]) == 1 && parts[0][0] >= 'A' && parts[0][0] <= 'Z' {
				// This is a drive letter path, not ADS
			} else {
				return newSecurityError("validate_windows_path", "alternate data streams not allowed")
			}
		}
	}

	// Check COM1-9 and LPT1-9
	if len(filename) == 4 && filename[3] >= '0' && filename[3] <= '9' {
		prefix := filename[:3]
		if prefix == "COM" || prefix == "LPT" {
			return newSecurityError("validate_windows_path", "Windows reserved device name")
		}
	}

	// Check COM0 and LPT0 (explicitly invalid in Windows)
	if filename == "COM0" || filename == "LPT0" {
		return newSecurityError("validate_windows_path", "Windows reserved device name")
	}

	// Check for invalid characters in Windows paths
	pathToCheck := absPath
	if len(absPath) > 2 && absPath[1] == ':' {
		pathToCheck = absPath[2:]
	}

	invalidChars := []string{"<", ">", ":", "\"", "|", "?", "*"}
	for _, char := range invalidChars {
		if strings.Contains(pathToCheck, char) {
			return newSecurityError("validate_windows_path", "invalid character in path")
		}
	}

	return nil
}
