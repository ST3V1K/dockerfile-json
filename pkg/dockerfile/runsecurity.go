//go:build dfrunsecurity

package dockerfile

import "github.com/moby/buildkit/frontend/dockerfile/instructions"

// Buildkit exports the GetSecurity function only when using -tags=dfrunsecurity.
// To avoid compilation errors, use a wrapper whose implementation calls GetSecurity
// only when using -tags=dfrunsecurity. See also ./runsecurity_stub.go
func getSecurity(cmd *instructions.RunCommand) string {
	return instructions.GetSecurity(cmd)
}
