package dockerfile

import (
	"fmt"
	"strings"
	"testing"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
)

func TestLabelExpansion(t *testing.T) {
	tests := []struct {
		name           string
		dockerfile     string
		buildArgs      map[string]string
		expectedLabels map[string]string
	}{
		{
			name: "simple variable expansion",
			dockerfile: `ARG VERSION=1.0.0
FROM alpine:3.10
LABEL version="$VERSION"`,
			buildArgs: map[string]string{},
			expectedLabels: map[string]string{
				"version": "1.0.0",
			},
		},
		{
			name: "complex expression with multiple variables",
			dockerfile: `ARG VERSION=1.0.0
ARG REVISION=abc123
FROM alpine:3.10
LABEL version="$VERSION+git-$REVISION"`,
			buildArgs: map[string]string{},
			expectedLabels: map[string]string{
				"version": "1.0.0+git-abc123",
			},
		},
		{
			name: "braced variable expansion",
			dockerfile: `ARG VERSION=1.0.0
ARG REVISION=abc123
FROM alpine:3.10
LABEL version="${VERSION}-${REVISION}"`,
			buildArgs: map[string]string{},
			expectedLabels: map[string]string{
				"version": "1.0.0-abc123",
			},
		},
		{
			name: "mixed variables and literals",
			dockerfile: `ARG VERSION=2.5.0
FROM alpine:3.10
LABEL description="App version $VERSION is ready"`,
			buildArgs: map[string]string{},
			expectedLabels: map[string]string{
				"description": "App version 2.5.0 is ready",
			},
		},
		{
			name: "override with build-arg",
			dockerfile: `ARG VERSION=1.0.0
ARG REVISION=default
FROM alpine:3.10
LABEL version="$VERSION+git-$REVISION"`,
			buildArgs: map[string]string{
				"REVISION": "xyz789",
			},
			expectedLabels: map[string]string{
				"version": "1.0.0+git-xyz789",
			},
		},
		{
			name: "multiple labels with expansion",
			dockerfile: `ARG VERSION=1.0.0
ARG REVISION=abc123
FROM alpine:3.10
LABEL version="$VERSION"
LABEL revision="$REVISION"
LABEL combined="$VERSION+git-$REVISION"`,
			buildArgs: map[string]string{},
			expectedLabels: map[string]string{
				"version":  "1.0.0",
				"revision": "abc123",
				"combined": "1.0.0+git-abc123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the dockerfile
			df, err := ParseReader(strings.NewReader(tt.dockerfile))
			if err != nil {
				t.Fatalf("Failed to parse dockerfile: %v", err)
			}

			// Create build arg expander
			env := func(key string) (string, error) {
				if val, ok := tt.buildArgs[key]; ok {
					return val, nil
				}
				// Return error if not found - this allows ARG defaults to be used
				return "", fmt.Errorf("not defined: $%s", key)
			}

			// Expand
			df.Expand(env)

			// Find LABEL commands and verify
			foundLabels := make(map[string]string)
			for _, stage := range df.Stages {
				for _, cmd := range stage.Commands {
					if cmd.Name == "LABEL" {
						labelCmd, ok := cmd.Command.(*instructions.LabelCommand)
						if !ok {
							t.Fatalf("Expected *instructions.LabelCommand, got %T", cmd.Command)
						}
						for _, label := range labelCmd.Labels {
							// Remove quotes if present
							value := strings.Trim(label.Value, "\"")
							foundLabels[label.Key] = value
						}
					}
				}
			}

			// Verify expected labels
			for key, expectedValue := range tt.expectedLabels {
				actualValue, found := foundLabels[key]
				if !found {
					t.Errorf("Label %q not found in output", key)
					continue
				}
				if actualValue != expectedValue {
					t.Errorf("Label %q: expected %q, got %q", key, expectedValue, actualValue)
				}
			}

			// Verify no extra labels
			for key := range foundLabels {
				if _, expected := tt.expectedLabels[key]; !expected {
					t.Errorf("Unexpected label %q with value %q", key, foundLabels[key])
				}
			}
		})
	}
}

func TestEnvExpansion(t *testing.T) {
	tests := []struct {
		name        string
		dockerfile  string
		buildArgs   map[string]string
		expectedEnv map[string]string
	}{
		{
			name: "ENV with variable expansion",
			dockerfile: `ARG VERSION=1.0.0
FROM alpine:3.10
ENV APP_VERSION=$VERSION`,
			buildArgs: map[string]string{},
			expectedEnv: map[string]string{
				"APP_VERSION": "1.0.0",
			},
		},
		{
			name: "ENV with complex expression",
			dockerfile: `ARG VERSION=1.0.0
ARG REVISION=abc123
FROM alpine:3.10
ENV FULL_VERSION="$VERSION+git-$REVISION"`,
			buildArgs: map[string]string{},
			expectedEnv: map[string]string{
				"FULL_VERSION": "1.0.0+git-abc123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the dockerfile
			df, err := ParseReader(strings.NewReader(tt.dockerfile))
			if err != nil {
				t.Fatalf("Failed to parse dockerfile: %v", err)
			}

			// Create build arg expander
			env := func(key string) (string, error) {
				if val, ok := tt.buildArgs[key]; ok {
					return val, nil
				}
				// Return error if not found - this allows ARG defaults to be used
				return "", fmt.Errorf("not defined: $%s", key)
			}

			// Expand
			df.Expand(env)

			// Find ENV commands and verify
			foundEnv := make(map[string]string)
			for _, stage := range df.Stages {
				for _, cmd := range stage.Commands {
					if cmd.Name == "ENV" {
						envCmd, ok := cmd.Command.(*instructions.EnvCommand)
						if !ok {
							t.Fatalf("Expected *instructions.EnvCommand, got %T", cmd.Command)
						}
						for _, kv := range envCmd.Env {
							// Remove quotes if present
							value := strings.Trim(kv.Value, "\"")
							foundEnv[kv.Key] = value
						}
					}
				}
			}

			// Verify expected env vars
			for key, expectedValue := range tt.expectedEnv {
				actualValue, found := foundEnv[key]
				if !found {
					t.Errorf("ENV %q not found in output", key)
					continue
				}
				if actualValue != expectedValue {
					t.Errorf("ENV %q: expected %q, got %q", key, expectedValue, actualValue)
				}
			}
		})
	}
}
