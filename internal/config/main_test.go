package config

import (
	"log"
	"os"
	"path/filepath"
	"testing"
)

// findProjectRoot searches for the project root directory by looking for the `go.mod` file.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Loop to check each parent directory
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); os.IsNotExist(err) {
			dir = filepath.Dir(dir)
			if dir == "/" || dir == "." {
				// Reached the filesystem root without finding go.mod
				return "", os.ErrNotExist
			}
		} else {
			// Found the go.mod file, so this directory is assumed to be the project root
			break
		}
	}
	return dir, nil
}

func TestMain(m *testing.M) {
	// Store the original working directory to restore later
	originalWD, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}

	// Use findProjectRoot to locate the project root directory
	projectRoot, err := findProjectRoot()
	if err != nil {
		log.Fatalf("Failed to find project root: %v", err)
	}

	// Change to the project root directory
	if err := os.Chdir(projectRoot); err != nil {
		log.Fatalf("Failed to change working directory to project root: %v", err)
	}

	// Your setup code here (e.g., initialize the logger)

	// Run the tests
	code := m.Run()

	// Your teardown code here (if necessary)

	// Restore the original working directory
	if err := os.Chdir(originalWD); err != nil {
		log.Fatalf("Failed to restore original working directory: %v", err)
	}

	// Exit with the return code from the test run
	os.Exit(code)
}
