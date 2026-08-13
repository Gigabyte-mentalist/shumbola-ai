package tools

import (
	"regexp"
	"strings"
)

// smbScriptHeader matches an NSE script result block's opening line,
// e.g. "| smb2-security-mode: " — the script name, nothing else on the
// line (its actual output follows on subsequent "|"/"|_"-prefixed lines).
// Nested fields within a block use the same "| name:" shape at deeper
// indentation (e.g. "|   3.1.1:"), so the whitespace after "|" is capped
// at one character — enough to distinguish a top-level script header
// from a nested field one level in, without needing to track indent
// depth generally.
var smbScriptHeader = regexp.MustCompile(`^\|\s?([a-zA-Z0-9_.-]+):\s*$`)

// parseSmbEnumOutput turns nmap's "Host script results:" section into one
// Finding per script that produced output. Unlike nuclei/ffuf, nmap's NSE
// report has no JSON form on stdout to parse — this is a plain-text
// scrape of the same block format nmap has used for its whole history:
// each script's output starts with a "| <script-name>:" header line and
// continues over subsequent "|"/"|_"-prefixed lines until the next
// header or the section ends (blank line / "Nmap done: ...").
func parseSmbEnumOutput(output string) []Finding {
	const marker = "Host script results:"
	idx := strings.Index(output, marker)
	if idx == -1 {
		return nil
	}

	var findings []Finding
	var name string
	var lines []string

	flush := func() {
		if name == "" || len(lines) == 0 {
			return
		}
		findings = append(findings, Finding{
			Title:    name,
			Severity: SeverityInfo,
			Detail:   strings.Join(lines, "\n"),
		})
	}

	for _, raw := range strings.Split(output[idx+len(marker):], "\n") {
		line := strings.TrimRight(raw, "\r")
		if m := smbScriptHeader.FindStringSubmatch(line); m != nil {
			flush()
			name = m[1]
			lines = nil
			continue
		}

		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			if name != "" {
				// Blank line or the next report section (e.g. "Nmap
				// done: ..."): the script-results block has ended.
				break
			}
			// Still skipping the blank line right after the "Host
			// script results:" marker itself, before the first header.
			continue
		}
		content := strings.TrimPrefix(trimmed, "|")
		content = strings.TrimPrefix(content, "_")
		content = strings.TrimSpace(content)
		if content != "" {
			lines = append(lines, content)
		}
	}
	flush()

	return findings
}
