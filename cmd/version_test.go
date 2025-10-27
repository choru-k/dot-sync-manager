package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestVersionCmd_Sanity(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "no arguments should work",
			args:        []string{},
			expectError: false,
		},
		{
			name:        "any arguments should error",
			args:        []string{"extra"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "version"}
			err := runVersion(cmd, tt.args)
			if (err != nil) != tt.expectError {
				t.Errorf("runVersion() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestGetVersionInfo(t *testing.T) {
	tests := []struct {
		name           string
		setupVersions  func()
		expectedFields int
	}{
		{
			name: "default dev values",
			setupVersions: func() {
				Version = "dev"
				Commit = "unknown"
				BuildDate = "unknown"
			},
			expectedFields: 3,
		},
		{
			name: "real version values",
			setupVersions: func() {
				Version = "v1.2.3"
				Commit = "abc123def456"
				BuildDate = "2024-01-15T10:30:00Z"
			},
			expectedFields: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupVersions()
			defer func() {
				// Reset to defaults
				Version = "dev"
				Commit = "unknown"
				BuildDate = "unknown"
			}()

			info := getVersionInfo()
			if len(info) != tt.expectedFields {
				t.Errorf("getVersionInfo() returned %d fields, expected %d", len(info), tt.expectedFields)
			}

			// Check that all expected fields are present
			if info["Version"] == "" {
				t.Error("expected Version field to be non-empty")
			}
			if info["Commit"] == "" {
				t.Error("expected Commit field to be non-empty")
			}
			if info["BuildDate"] == "" {
				t.Error("expected BuildDate field to be non-empty")
			}
		})
	}
}

func TestFormatVersionOutput(t *testing.T) {
	tests := []struct {
		name           string
		versionInfo    map[string]string
		expectedFields []string
	}{
		{
			name: "standard version info",
			versionInfo: map[string]string{
				"Version":   "v1.2.3",
				"Commit":    "abc123def456",
				"BuildDate": "2024-01-15T10:30:00Z",
			},
			expectedFields: []string{"v1.2.3", "abc123def456", "2024-01-15T10:30:00Z"},
		},
		{
			name: "dev version info",
			versionInfo: map[string]string{
				"Version":   "dev",
				"Commit":    "unknown",
				"BuildDate": "unknown",
			},
			expectedFields: []string{"dev", "unknown", "unknown"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatVersionOutput(tt.versionInfo)

			for _, expectedField := range tt.expectedFields {
				if !strings.Contains(output, expectedField) {
					t.Errorf("expected output to contain %q, got: %s", expectedField, output)
				}
			}
		})
	}
}

func TestVersionCmd_Integration(t *testing.T) {
	// Test version command with actual execution
	cmd := &cobra.Command{Use: "version"}

	// Set test version values
	originalVersion := Version
	originalCommit := Commit
	originalBuildDate := BuildDate
	defer func() {
		Version = originalVersion
		Commit = originalCommit
		BuildDate = originalBuildDate
	}()

	Version = "test-v1.0.0"
	Commit = "test-commit-123"
	BuildDate = "2024-01-15"

	err := runVersion(cmd, []string{})
	if err != nil {
		t.Errorf("runVersion() failed: %v", err)
	}
}

func TestVersionVariables(t *testing.T) {
	// Test that version variables can be set and retrieved
	originalVersion := Version
	originalCommit := Commit
	originalBuildDate := BuildDate
	defer func() {
		Version = originalVersion
		Commit = originalCommit
		BuildDate = originalBuildDate
	}()

	// Test setting values
	Version = "v2.0.0"
	Commit = "def456abc123"
	BuildDate = "2024-02-20T14:45:30Z"

	if Version != "v2.0.0" {
		t.Errorf("expected Version to be v2.0.0, got %s", Version)
	}
	if Commit != "def456abc123" {
		t.Errorf("expected Commit to be def456abc123, got %s", Commit)
	}
	if BuildDate != "2024-02-20T14:45:30Z" {
		t.Errorf("expected BuildDate to be 2024-02-20T14:45:30Z, got %s", BuildDate)
	}
}

func TestVersionOutputFormatting(t *testing.T) {
	// Test different version output formats
	testCases := []struct {
		name    string
		version string
		commit  string
		date    string
	}{
		{
			name:    "semantic version",
			version: "v1.2.3",
			commit:  "abc123",
			date:    "2024-01-15",
		},
		{
			name:    "development version",
			version: "dev",
			commit:  "unknown",
			date:    "unknown",
		},
		{
			name:    "release candidate",
			version: "v2.0.0-rc1",
			commit: "def456",
			date:    "2024-02-20",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			info := map[string]string{
				"Version":   tc.version,
				"Commit":    tc.commit,
				"BuildDate": tc.date,
			}

			output := formatVersionOutput(info)

			// Verify all fields are present in output
			if !strings.Contains(output, tc.version) {
				t.Errorf("expected output to contain version %s", tc.version)
			}
			if !strings.Contains(output, tc.commit) {
				t.Errorf("expected output to contain commit %s", tc.commit)
			}
			if !strings.Contains(output, tc.date) {
				t.Errorf("expected output to contain date %s", tc.date)
			}
		})
	}
}

func TestVersionCommandFlags(t *testing.T) {
	// Test version command with various flag combinations
	cmd := &cobra.Command{Use: "version"}

	// Add test flags if needed
	cmd.Flags().Bool("short", false, "Show short version")
	cmd.Flags().Bool("build-info", false, "Show build info only")

	tests := []struct {
		name        string
		flags       map[string]bool
		expectError bool
	}{
		{
			name:        "no flags",
			flags:       map[string]bool{},
			expectError: false,
		},
		{
			name:        "short flag",
			flags:       map[string]bool{"short": true},
			expectError: false,
		},
		{
			name:        "build-info flag",
			flags:       map[string]bool{"build-info": true},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags
			cmd.Flags().VisitAll(func(f *pflag.Flag) {
				if err := f.Value.Set(f.DefValue); err != nil {
					t.Logf("warning: failed to reset flag %s: %v", f.Name, err)
				}
			})

			// Set test flags
			for flagName, value := range tt.flags {
				flagValue := "false"
				if value {
					flagValue = "true"
				}
				if err := cmd.Flags().Set(flagName, flagValue); err != nil {
					t.Fatalf("failed to set flag %s: %v", flagName, err)
				}
			}

			err := runVersion(cmd, []string{})
			if (err != nil) != tt.expectError {
				t.Errorf("runVersion() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

