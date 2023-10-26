package config

import (
	"os"
	"testing"
)

// TestMain serves as the entry point for running all unit tests in this package.
// It performs global setup tasks such as initializing the logger by importing
// the custom logging package. Additional setup or teardown operations can be
// added as needed. After setup, TestMain runs all tests in the package and then
// executes any required teardown before exiting.
func TestMain(m *testing.M) {
	// Your setup code here

	// Run tests
	code := m.Run()

	// Your teardown code here

	// Exit
	os.Exit(code)
}
