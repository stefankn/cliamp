package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFinchSection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	path := filepath.Join(os.Getenv("HOME"), ".config", "cliamp", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	data := []byte(`
[finch]
url = "https://finch.example.com"
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Finch.URL != "https://finch.example.com" {
		t.Fatalf("URL = %q, want https://finch.example.com", cfg.Finch.URL)
	}
	if !cfg.Finch.IsSet() {
		t.Fatal("IsSet() = false, want true")
	}
}

func TestFinchIsSet(t *testing.T) {
	tests := []struct {
		name string
		cfg  FinchConfig
		want bool
	}{
		{name: "empty", cfg: FinchConfig{}, want: false},
		{name: "url", cfg: FinchConfig{URL: "https://finch"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.IsSet(); got != tt.want {
				t.Fatalf("IsSet() = %v, want %v", got, tt.want)
			}
		})
	}
}
