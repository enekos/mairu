package desktop

import (
	"runtime"
	"testing"
)

func TestDetectAssetName(t *testing.T) {
	name := detectAssetName()
	if name == "" {
		t.Fatal("detectAssetName returned empty string")
	}

	switch runtime.GOOS + "-" + runtime.GOARCH {
	case "darwin-arm64":
		if name != "meilisearch-macos-apple-silicon" {
			t.Fatalf("expected meilisearch-macos-apple-silicon, got %s", name)
		}
	case "darwin-amd64":
		if name != "meilisearch-macos-amd64" {
			t.Fatalf("expected meilisearch-macos-amd64, got %s", name)
		}
	case "linux-amd64":
		if name != "meilisearch-linux-amd64" {
			t.Fatalf("expected meilisearch-linux-amd64, got %s", name)
		}
	case "linux-arm64":
		if name != "meilisearch-linux-aarch64" {
			t.Fatalf("expected meilisearch-linux-aarch64, got %s", name)
		}
	}
}

func TestFreePort(t *testing.T) {
	port, err := freePort()
	if err != nil {
		t.Fatalf("freePort failed: %v", err)
	}
	if port < 1024 || port > 65535 {
		t.Fatalf("unexpected port: %d", port)
	}
}
