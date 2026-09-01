package buildargs

import (
	"fmt"
	"os"
	"strings"
)

// ParseBuildArgFile reads a build arg file and returns a map of key-value pairs.
// The file format is one argument per line in the form "KEY=VALUE".
// Blank lines and lines starting with '#' are ignored.
func ParseBuildArgFile(buildArgFilePath string) (map[string]string, error) {
	buildArgFile, err := os.ReadFile(buildArgFilePath)
	if err != nil {
		return nil, err
	}

	args := make(map[string]string)

	for i, line := range strings.Split(string(buildArgFile), "\n") {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			return nil, fmt.Errorf("%s:%d: expected arg=value, got %q", buildArgFilePath, i+1, line)
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		if key == "" {
			return nil, fmt.Errorf("%s:%d: empty key in %q", buildArgFilePath, i+1, line)
		}

		// Later definitions override earlier ones
		args[key] = value
	}

	return args, nil
}
