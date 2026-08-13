package tools

import (
	"fmt"
	"regexp"
	"strings"
)

// openPortLine matches nmap's normal-output port table rows, e.g.:
//
//	22/tcp   open  ssh     OpenSSH 8.9p1 Ubuntu
//	80/tcp   open  http    nginx 1.24.0
var openPortLine = regexp.MustCompile(`^(\d+)/(tcp|udp)\s+open\s+(\S+)(?:\s+(.*))?$`)

// parseNmapOutput extracts one Finding per open port from nmap's default
// (normal) text output. Closed/filtered ports are not reported — TZ
// requires only proof-backed observations to reach the task tree.
func parseNmapOutput(output string) []Finding {
	var findings []Finding
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		m := openPortLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		port, proto, service, version := m[1], m[2], m[3], strings.TrimSpace(m[4])

		title := fmt.Sprintf("%s/%s open (%s)", port, proto, service)
		detail := fmt.Sprintf("Port %s/%s is open, service detected as %q", port, proto, service)
		if version != "" {
			detail += fmt.Sprintf(", version info: %q", version)
		}

		findings = append(findings, Finding{
			Title:    title,
			Severity: SeverityInfo,
			Detail:   detail,
		})
	}
	return findings
}
