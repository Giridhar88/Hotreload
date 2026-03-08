package main

import "testing"

func TestShouldWatch(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"main.go", true},
		{"cmd/server/main.go", true},
		{"config.yaml", true},
		{"templates/index.html", true},
		{".git/HEAD", false},
		{".git/objects/pack/abc123", false},
		{"node_modules/pkg/index.js", false},
		{"vendor/lib/lib.go", false},
		{".idea/workspace.xml", false},
		{".vscode/settings.json", false},
		{"main.go~", false},
		{".main.go.swp", false},
		{"#main.go#", false},
		{".#main.go", false},
		{"server.exe", false},
		{"lib.so", false},
		{"lib.a", false},
		{"pkg.test", false},
		{".env", false},
		{".DS_Store", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := shouldWatch(tt.path); got != tt.want {
				t.Errorf("shouldWatch(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
