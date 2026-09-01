package buildargs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseBuildArgFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expected    map[string]string
		expectError bool
		errorSubstr string
	}{
		{
			name: "valid args",
			content: `FOO=bar
BAZ=qux`,
			expected: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
			expectError: false,
		},
		{
			name: "with comments and blank lines",
			content: `# This is a comment
FOO=bar

# Another comment
BAZ=qux

`,
			expected: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
			expectError: false,
		},
		{
			name: "with whitespace",
			content: `  FOO  =  bar
  BAZ=qux`,
			expected: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
			expectError: false,
		},
		{
			name: "empty value",
			content: `FOO=
BAZ=qux`,
			expected: map[string]string{
				"FOO": "",
				"BAZ": "qux",
			},
			expectError: false,
		},
		{
			name: "value with equals sign",
			content: `FOO=bar=baz
URL=https://example.com?param=value`,
			expected: map[string]string{
				"FOO": "bar=baz",
				"URL": "https://example.com?param=value",
			},
			expectError: false,
		},
		{
			name: "override duplicate keys",
			content: `FOO=first
FOO=second
FOO=third`,
			expected: map[string]string{
				"FOO": "third",
			},
			expectError: false,
		},
		{
			name: "missing equals sign",
			content: `FOO=bar
INVALID_LINE
BAZ=qux`,
			expectError: true,
			errorSubstr: "expected arg=value",
		},
		{
			name:        "equals at start",
			content:     `=value`,
			expectError: true,
			errorSubstr: "expected arg=value",
		},
		{
			name:        "empty key with whitespace",
			content:     `  = value`,
			expectError: true,
			errorSubstr: "expected arg=value",
		},
		{
			name:        "empty file",
			content:     "",
			expected:    map[string]string{},
			expectError: false,
		},
		{
			name: "only comments and blank lines",
			content: `# Comment 1
# Comment 2

`,
			expected:    map[string]string{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "argfile.conf")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
			result, err := ParseBuildArgFile(tmpFile)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorSubstr != "" && !strings.Contains(err.Error(), tt.errorSubstr) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorSubstr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("Result mismatch (-expected +actual):\n%s", diff)
			}
		})
	}
}

func TestParseBuildArgFile_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	nonexistent := filepath.Join(tmpDir, "nonexistent.txt")

	_, err := ParseBuildArgFile(nonexistent)
	if err == nil {
		t.Fatal("Expected error for missing file, got nil")
	}

	if !strings.Contains(err.Error(), "no such file or directory") {
		t.Errorf("Expected 'no such file or directory' error, got: %v", err)
	}
}
