package tools

import (
	"fmt"
	"strings"
)

// validateTarget is the shared, defensive target check every wrapper
// uses: non-empty and free of shell metacharacters. Cmd is always passed
// as an argv array (never through a shell), so this is belt-and-braces
// against any future change that might interpolate a target into a
// shell string.
func validateTarget(toolName, target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("%s: target must not be empty", toolName)
	}
	if strings.ContainsAny(target, ";&|`$()<>\"' \t\n") {
		return fmt.Errorf("%s: target contains disallowed characters: %q", toolName, target)
	}
	return nil
}
