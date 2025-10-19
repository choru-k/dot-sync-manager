package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Pattern represents a single ignore pattern
type Pattern struct {
	raw     string
	negated bool
	dirOnly bool
}

// Parser handles parsing and matching of ignore patterns
type Parser struct {
	patterns []Pattern
	root     string
}

// New creates a new ignore parser
func New(root string) *Parser {
	return &Parser{
		root:     root,
		patterns: make([]Pattern, 0),
	}
}

// LoadFromFile loads ignore patterns from a file
func (p *Parser) LoadFromFile(filename string) (err error) {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File not found is OK
		}
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		pattern := p.parsePattern(line)
		p.patterns = append(p.patterns, pattern)
	}

	if scanErr := scanner.Err(); scanErr != nil && err == nil {
		err = scanErr
	}
	return err
}

// parsePattern parses a single pattern line
func (p *Parser) parsePattern(line string) Pattern {
	pattern := Pattern{
		negated: false,
		dirOnly: false,
		raw:     line,
	}

	// Handle negation
	if strings.HasPrefix(line, "!") {
		pattern.negated = true
		line = line[1:]
	}

	// Handle directory-only patterns
	if strings.HasSuffix(line, "/") {
		pattern.dirOnly = true
		line = line[:len(line)-1]
	}

	pattern.raw = line
	return pattern
}

// Match checks if a path matches any of the ignore patterns
func (p *Parser) Match(path string) bool {
	// Convert to relative path from root
	relPath := path
	if p.root != "" {
		if filepath.IsAbs(path) {
			// Convert absolute path to relative
			if absRoot, err := filepath.Abs(p.root); err == nil {
				if rel, err := filepath.Rel(absRoot, path); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
					relPath = rel
				} else {
					// Path is outside root, don't match
					return false
				}
			}
		} else {
			// Already relative, but ensure it doesn't go outside root
			cleanPath := filepath.Clean(path)
			if strings.HasPrefix(cleanPath, "../") || cleanPath == ".." {
				// Path goes outside root, don't match
				return false
			}
			relPath = cleanPath
		}
	}

	// Normalize path separators
	relPath = filepath.ToSlash(relPath)

	// Handle empty path
	if relPath == "." || relPath == "" {
		relPath = ""
	}

	var ignored bool

	// Check each pattern in order
	// Later patterns override earlier ones (including negations)
	for _, pattern := range p.patterns {
		if p.matchPattern(pattern, relPath) {
			ignored = !pattern.negated
		}
	}

	return ignored
}

// matchPattern checks if a single pattern matches a path
func (p *Parser) matchPattern(pattern Pattern, path string) bool {
	patternStr := pattern.raw

	// Handle double-asterisk patterns first
	if strings.Contains(patternStr, "**") {
		return p.matchDoubleAsterisk(patternStr, path)
	}

	// For directory-only patterns, check if path is the directory or under it
	if pattern.dirOnly {
		// If path is exactly the directory name
		if path == patternStr {
			return true
		}
		// If path is under the directory
		if strings.HasPrefix(path, patternStr+"/") {
			return true
		}
		// If pattern has no path separators, check if any path component matches
		if !strings.Contains(patternStr, "/") {
			pathParts := strings.Split(path, "/")
			for _, part := range pathParts {
				if part == patternStr {
					return true
				}
			}
		}
		return false
	}

	// Try simple filepath.Match first (only if pattern has no slashes)
	if !strings.Contains(patternStr, "/") {
		// For patterns without slashes, check against each path component
		pathParts := strings.Split(path, "/")
		for _, part := range pathParts {
			if matched, _ := filepath.Match(patternStr, part); matched {
				return true
			}
		}
		// Also try matching against the full path
		if matched, _ := filepath.Match(patternStr, path); matched {
			return true
		}
	}

	// Handle more complex patterns
	return p.matchAdvanced(patternStr, path)
}

// matchDoubleAsterisk handles ** patterns
func (p *Parser) matchDoubleAsterisk(pattern, path string) bool {
	// Pattern: **/temp matches temp, dir/temp, deep/nested/temp
	if strings.HasPrefix(pattern, "**/") {
		suffix := pattern[3:] // Remove **/
		// Check if path ends with suffix
		if strings.HasSuffix(path, suffix) {
			// Ensure it's a full path component match
			pathWithoutSuffix := path[:len(path)-len(suffix)]
			if pathWithoutSuffix == "" || strings.HasSuffix(pathWithoutSuffix, "/") {
				return true
			}
		}
		// Check if any directory component matches
		pathParts := strings.Split(path, "/")
		for _, part := range pathParts {
			if matched, _ := filepath.Match(suffix, part); matched {
				return true
			}
		}
	}

	// Pattern: logs/** matches logs, logs/app.log, logs/2023/01/app.log
	if strings.HasSuffix(pattern, "/**") {
		prefix := pattern[:len(pattern)-3] // Remove /**
		// Match the directory itself or anything under it
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}

	// Pattern: dir/**/file matches dir/file, dir/sub/file, dir/a/b/c/file
	if idx := strings.Index(pattern, "/**/"); idx != -1 {
		prefix := pattern[:idx]
		suffix := pattern[idx+4:]

		// Path must start with prefix
		if !strings.HasPrefix(path, prefix+"/") && path != prefix {
			return false
		}

		// Path must end with suffix
		if suffix == "" {
			return true // dir/** matches dir and everything under it
		}

		// Check if path ends with suffix
		if strings.HasSuffix(path, suffix) {
			// Extract the middle part between prefix and suffix
			middle := path[len(prefix) : len(path)-len(suffix)]
			// Middle can be empty or contain any path
			if middle == "" || strings.HasPrefix(middle, "/") && strings.HasSuffix(middle, "/") {
				return true
			}
			if middle == "/" {
				return true // dir/file case
			}
			// Any path like dir/sub/file or dir/a/b/c/file
			if strings.HasPrefix(middle, "/") {
				return true
			}
		}
	}

	return false
}

// matchAdvanced handles advanced pattern matching
func (p *Parser) matchAdvanced(pattern, path string) bool {
	// Split path into parts
	pathParts := strings.Split(path, "/")
	patternParts := strings.Split(pattern, "/")

	// Handle patterns with wildcards
	return p.matchParts(patternParts, pathParts)
}

// matchParts matches pattern parts against path parts
func (p *Parser) matchParts(patternParts, pathParts []string) bool {
	// Simple case: single pattern part
	if len(patternParts) == 1 {
		return p.matchSinglePattern(patternParts[0], pathParts)
	}

	// Multi-part pattern
	return p.matchMultiPattern(patternParts, pathParts)
}

// matchSinglePattern matches a single pattern against path parts
func (p *Parser) matchSinglePattern(pattern string, pathParts []string) bool {
	for _, part := range pathParts {
		if matched, _ := filepath.Match(pattern, part); matched {
			return true
		}
	}
	return false
}

// matchMultiPattern matches multi-part patterns
func (p *Parser) matchMultiPattern(patternParts, pathParts []string) bool {
	// If pattern has more parts than path, it can't match
	if len(patternParts) > len(pathParts) {
		return false
	}

	// Try to match the pattern at the end of the path
	offset := len(pathParts) - len(patternParts)
	for i := 0; i < len(patternParts); i++ {
		patternPart := patternParts[i]
		pathPart := pathParts[offset+i]

		// Handle ** wildcard
		if patternPart == "**" {
			continue // ** matches anything
		}

		matched, err := filepath.Match(patternPart, pathPart)
		if err != nil || !matched {
			return false
		}
	}

	return true
}

// GetPatterns returns the current patterns (for testing)
func (p *Parser) GetPatterns() []Pattern {
	return p.patterns
}

// Clear removes all patterns
func (p *Parser) Clear() {
	p.patterns = make([]Pattern, 0)
}
