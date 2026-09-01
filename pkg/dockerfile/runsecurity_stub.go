//go:build !dfrunsecurity

package dockerfile

import "github.com/moby/buildkit/frontend/dockerfile/instructions"

// Buildkit exports the GetSecurity function only when using -tags=dfrunsecurity.
// When not using the tag, do not attempt to call the function and just return an
// empty string. A Dockerfile that includes a 'RUN --security' instruction will
// cause a parse error anyway. See also ./runsecurity.go
func getSecurity(cmd *instructions.RunCommand) string {
	return ""
}
