package logging

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupWritesStandardLoggerToFile(t *testing.T) {
	originalOutput := log.Writer()
	defer log.SetOutput(originalOutput)

	path := filepath.Join(t.TempDir(), "seller-service.log")
	file, err := Setup(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	log.Print("hello file logger")
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "hello file logger") {
		t.Fatalf("log content mismatch: %s", string(content))
	}
}
