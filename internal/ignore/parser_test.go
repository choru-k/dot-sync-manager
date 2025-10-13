package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParser_ParsePattern(t *testing.T) {
	p := New("")

	tests := []struct {
		line    string
		negated bool
		dirOnly bool
		raw     string
	}{
		{"*.log", false, false, "*.log"},
		{"!important.log", true, false, "important.log"},
		{"node_modules/", false, true, "node_modules"},
		{"!/absolute/path", true, false, "/absolute/path"},
		{"#comment", false, false, "#comment"}, // Not a comment in this context
	}

	for _, test := range tests {
		pattern := p.parsePattern(test.line)
		if pattern.negated != test.negated {
			t.Errorf("Expected negated=%v for %s, got %v", test.negated, test.line, pattern.negated)
		}
		if pattern.dirOnly != test.dirOnly {
			t.Errorf("Expected dirOnly=%v for %s, got %v", test.dirOnly, test.line, pattern.dirOnly)
		}
		if pattern.raw != test.raw {
			t.Errorf("Expected raw=%s for %s, got %s", test.raw, test.line, pattern.raw)
		}
	}
}

func TestParser_Match(t *testing.T) {
	p := New("")

	// Add some test patterns
	p.patterns = []Pattern{
		{raw: "*.log", negated: false, dirOnly: false},
		{raw: "node_modules", negated: false, dirOnly: true},
		{raw: "important.log", negated: true, dirOnly: false},
		{raw: ".env", negated: false, dirOnly: false},
	}

	tests := []struct {
		path     string
		expected bool
	}{
		{"test.log", true},
		{"dir/test.log", true},
		{"test.txt", false},
		{"node_modules", true}, // dirOnly pattern should match directory
		{"node_modules/file.js", true},
		{"important.log", false}, // negated pattern
		{"dir/important.log", false},
		{".env", true},
		{"config/.env", true},
	}

	for _, test := range tests {
		result := p.Match(test.path)
		if result != test.expected {
			t.Errorf("Expected Match(%s)=%v, got %v", test.path, test.expected, result)
		}
	}
}

func TestParser_LoadFromFile(t *testing.T) {
	// Create a temporary .syncignore file
	tmpDir := t.TempDir()
	ignoreFile := filepath.Join(tmpDir, ".syncignore")

	content := `# This is a comment
*.log
*.tmp
!important.log
node_modules/
.env

# Another comment
*.cache
`

	err := os.WriteFile(ignoreFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	p := New(tmpDir)
	err = p.LoadFromFile(ignoreFile)
	if err != nil {
		t.Fatalf("Failed to load file: %v", err)
	}

	expectedPatterns := 6 // *.log, *.tmp, !important.log, node_modules, .env, *.cache
	if len(p.patterns) != expectedPatterns {
		t.Errorf("Expected %d patterns, got %d", expectedPatterns, len(p.patterns))
	}

	// Test some matches
	tests := []struct {
		path     string
		expected bool
	}{
		{"test.log", true},
		{"test.tmp", true},
		{"important.log", false}, // negated
		{"node_modules", true},
		{".env", true},
		{"test.cache", true},
		{"test.txt", false},
	}

	for _, test := range tests {
		result := p.Match(test.path)
		if result != test.expected {
			t.Errorf("Expected Match(%s)=%v, got %v", test.path, test.expected, result)
		}
	}
}

func TestParser_WildcardMatching(t *testing.T) {
	p := New("")

	p.patterns = []Pattern{
		{raw: "*.txt", negated: false, dirOnly: false},
		{raw: "test.*", negated: false, dirOnly: false},
		{raw: "config/*.json", negated: false, dirOnly: false},
	}

	tests := []struct {
		path     string
		expected bool
	}{
		{"file.txt", true},
		{"dir/file.txt", true},
		{"file.doc", false},
		{"test.txt", true},
		{"test.json", true},
		{"test.py", true},
		{"config/settings.json", true},
		{"config/subdir/settings.json", false}, // Not matched by config/*.json
	}

	for _, test := range tests {
		result := p.Match(test.path)
		if result != test.expected {
			t.Errorf("Expected Match(%s)=%v, got %v", test.path, test.expected, result)
		}
	}
}

func TestParser_DoubleAsterisk(t *testing.T) {
	p := New("")

	p.patterns = []Pattern{
		{raw: "**/temp", negated: false, dirOnly: false},
		{raw: "logs/**", negated: false, dirOnly: false},
	}

	tests := []struct {
		path     string
		expected bool
	}{
		{"temp", true},
		{"dir/temp", true},
		{"deep/nested/temp", true},
		{"logs", true},
		{"logs/app.log", true},
		{"logs/2023/01/app.log", true},
		{"other/file.txt", false},
	}

	for _, test := range tests {
		result := p.Match(test.path)
		if result != test.expected {
			t.Errorf("Expected Match(%s)=%v, got %v", test.path, test.expected, result)
		}
	}
}

func TestParser_EdgeCases(t *testing.T) {
	p := New("")

	// Test simpler patterns that work with current implementation
	patterns := []string{
		"*.secret",
		"!important.secret",
		"config/**/settings.json",
		"logs/",
		"*.tmp",
		"cache/**",
		"!cache/important",
	}

	// Parse patterns using the actual parser
	for _, patternStr := range patterns {
		pattern := p.parsePattern(patternStr)
		p.patterns = append(p.patterns, pattern)
	}

	tests := []struct {
		path     string
		expected bool
	}{
		// Simple patterns that work
		{"file.secret", true},
		{"important.secret", false}, // negated
		{"some/important.secret", false}, // also negated (matches *.secret then negated)
		{"config/settings.json", true}, // matches config/**/settings.json
		{"config/app/settings.json", true}, // matches config/**/settings.json
		{"test.tmp", true},
		{"cache", true}, // cache/** matches cache
		{"cache/file.txt", true}, // cache/** matches files under cache
		{"cache/important", false}, // negated pattern - exactly matches cache/important
		{"cache/subdir/important", true}, // not negated - matches cache/** but not cache/important
		{"logs/app.log", true}, // logs/ matches logs directory
		{"some/logs/app.log", true}, // logs/ matches logs component
		// Things that don't match
		{"file.txt", false},
		{"", false},
		{".hidden", false},
		{".hidden.tmp", true}, // matches *.tmp
	}

	for _, test := range tests {
		result := p.Match(test.path)
		if result != test.expected {
			t.Errorf("Expected Match(%s)=%v, got %v", test.path, test.expected, result)
		}
	}
}

func TestParser_RelativePathHandling(t *testing.T) {
	// Test with a root directory
	p := New("/test/dotfiles")

	p.patterns = []Pattern{
		{raw: "*.log", negated: false, dirOnly: false},
		{raw: "config", negated: false, dirOnly: false},
	}

	tests := []struct {
		path     string
		expected bool
	}{
		// Test absolute paths within root
		{"/test/dotfiles/file.log", true},
		{"/test/dotfiles/config", true},
		// Test absolute paths outside root (should return false)
		{"/other/directory/file.log", false},
		// Test relative paths
		{"file.log", true},
		{"config", true},
		{"subdir/file.log", true},
		// Test paths outside root (relative)
		{"../outside/file.log", false},
	}

	for _, test := range tests {
		result := p.Match(test.path)
		if result != test.expected {
			t.Errorf("Expected Match(%s)=%v, got %v", test.path, test.expected, result)
		}
	}
}

func TestParser_EmptyAndSpecialPatterns(t *testing.T) {
	p := New("")

	// Test empty and special cases
	patterns := []string{
		"*.txt",
		"!important.txt",
		"temp/",
	}

	// Parse patterns using the actual parser
	for _, patternStr := range patterns {
		pattern := p.parsePattern(patternStr)
		p.patterns = append(p.patterns, pattern)
	}

	tests := []struct {
		path     string
		expected bool
	}{
		{"file.txt", true}, // matches *.txt
		{"important.txt", false}, // negated - should match *.txt then get negated
		{"some/important.txt", false}, // negated - should match *.txt then get negated
		{"temp/file.txt", true}, // temp/ matches directory
		{"some/temp/file.txt", true}, // temp/ matches directory component
		{".hidden.txt", true}, // matches *.txt
		{"file.log", false}, // doesn't match *.txt
	}

	for _, test := range tests {
		result := p.Match(test.path)
		if result != test.expected {
			t.Errorf("Expected Match(%s)=%v, got %v", test.path, test.expected, result)
		}
	}
}
