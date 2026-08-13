// Package standards maps tool findings to the report standards TZ
// section 5 requires (OWASP Top 10, MITRE ATT&CK, CVSS).
//
// There is no CVE database or RAG knowledge base wired in yet (that's
// Bosqich 4, TZ 8-bo'lim), so every mapping here is a coarse, tool/
// keyword-based heuristic — not a real vulnerability classification.
// CVSSScore in particular is NOT a computed CVSS vector score; it's a
// representative severity-band number so the report has something
// numeric to show. Both are meant to be replaced once a real knowledge
// base is available, without changing report.Build's call sites.
package standards

import (
	"strings"

	"github.com/shumbola/agent-core/internal/tools"
)

// OWASPCategory returns a best-effort OWASP Top 10 (2021) category for a
// finding, based on which tool produced it and keywords in its title.
func OWASPCategory(tool, title string) string {
	lowerTitle := strings.ToLower(title)
	switch tool {
	case "nuclei":
		if strings.Contains(lowerTitle, "cve-") {
			return "A06:2021 - Vulnerable and Outdated Components"
		}
		return "A04:2021 - Insecure Design"
	case "ffuf":
		return "A01:2021 - Broken Access Control"
	case "testssl":
		return "A02:2021 - Cryptographic Failures"
	case "nmap":
		return "A05:2021 - Security Misconfiguration"
	default:
		return "A05:2021 - Security Misconfiguration"
	}
}

// MITRETechnique returns a best-effort MITRE ATT&CK technique ID
// associated with the tool/finding type.
func MITRETechnique(tool, title string) string {
	switch tool {
	case "nmap":
		return "T1046 - Network Service Discovery"
	case "nuclei":
		return "T1190 - Exploit Public-Facing Application"
	case "ffuf":
		return "T1595.003 - Active Scanning: Wordlist Scanning"
	case "testssl":
		return "T1040 - Network Sniffing"
	default:
		return "T1595 - Active Scanning"
	}
}

// CVSSScore returns a representative CVSS-like numeric score for a
// severity band. This is NOT a real CVSS vector calculation — it exists
// so the report has a consistent number to sort/display by until real
// CVE-based scoring lands.
func CVSSScore(sev tools.Severity) float64 {
	switch sev {
	case tools.SeverityCritical:
		return 9.5
	case tools.SeverityHigh:
		return 7.5
	case tools.SeverityMed:
		return 5.5
	case tools.SeverityLow:
		return 3.0
	default:
		return 0.0
	}
}

// Remediation returns a generic tuzatish tavsiyasi (remediation
// suggestion) for the given tool's finding type.
func Remediation(tool string) string {
	switch tool {
	case "nmap":
		return "Ishlatilmayotgan yoki keraksiz tarmoq xizmatlarini o'chiring, qolganlarini firewall/tarmoq segmentatsiyasi bilan cheklang."
	case "nuclei":
		return "Aniqlangan zaif komponent yoki konfiguratsiyani ishlab chiqaruvchining eng so'nggi xavfsiz versiyasiga yangilang."
	case "ffuf":
		return "Ochib qolgan yo'l/panelni autentifikatsiya bilan himoyalang yoki tashqi tarmoqdan foydalanib bo'lmaydigan qiling."
	case "testssl":
		return "Zaif/eskirgan TLS protokol va shifrlarni o'chirib, faqat zamonaviy TLS 1.2+ va kuchli shifr to'plamlarini yoqing."
	default:
		return "Topilmani ko'rib chiqib, tegishli xavfsizlik konfiguratsiyasini yangilang."
	}
}
