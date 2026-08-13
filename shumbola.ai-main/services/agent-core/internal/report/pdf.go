package report

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/go-pdf/fpdf"
)

// unicodeFontFamily is the font family name registered with fpdf when a
// Cyrillic-capable TTF is available.
const unicodeFontFamily = "DejaVu"

// RenderPDF renders data as a PDF report. fontPath, if non-empty and
// readable, must point to a Unicode TrueType font with Cyrillic glyph
// coverage (e.g. DejaVu Sans) — required for uz-Cyrl/ru text to render
// correctly, since fpdf's built-in core fonts (Helvetica/Times/Courier)
// only cover Latin-1 (cp1252) and expect input in that encoding, not
// UTF-8. If fontPath is empty or unreadable, RenderPDF falls back to the
// core Helvetica font (via a UTF-8→cp1252 translator, so Latin-script
// punctuation like em dashes still renders correctly) and logs a
// warning: the PDF is still produced, but Cyrillic glyphs outside
// cp1252 will be dropped. In the shipped Docker image, fontPath is
// expected to point at the `ttf-dejavu` package's font file (see
// Dockerfile).
func RenderPDF(data ReportData, fontPath string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)

	fontFamily := "Helvetica"
	tr := pdf.UnicodeTranslatorFromDescriptor("") // cp1252, matches the core fonts
	if fontPath != "" {
		if _, err := os.Stat(fontPath); err == nil {
			pdf.AddUTF8Font(unicodeFontFamily, "", fontPath)
			fontFamily = unicodeFontFamily
			tr = func(s string) string { return s } // AddUTF8Font fonts take UTF-8 directly
		} else {
			log.Printf("report: Cyrillic font not found at %q, falling back to Helvetica (uz-Cyrl/ru text may render incorrectly): %v", fontPath, err)
		}
	} else {
		log.Printf("report: no font path configured, falling back to Helvetica (uz-Cyrl/ru text may render incorrectly)")
	}

	w := writer{pdf: pdf, family: fontFamily, tr: tr, lang: data.Lang}

	pdf.AddPage()
	w.titlePage(data)
	w.executiveSummary(data)
	w.findings(data)
	w.localStandardsPlaceholder(data)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("report: render pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// writer bundles the fpdf document with the font family and encoding
// translator every write helper needs, so callers don't have to thread
// three parameters through each function individually.
type writer struct {
	pdf    *fpdf.Fpdf
	family string
	tr     func(string) string
	lang   string
}

func (w writer) setFont(size float64) {
	w.pdf.SetFont(w.family, "", size)
}

func (w writer) line(h float64, text string) {
	w.pdf.MultiCell(0, h, w.tr(text), "", "L", false)
}

func (w writer) label(key string) string {
	return label(w.lang, key)
}

func (w writer) titlePage(data ReportData) {
	w.setFont(20)
	w.line(10, w.label("title"))
	w.pdf.Ln(4)

	w.setFont(11)
	w.line(7, fmt.Sprintf("%s: %s", w.label("engagement"), data.EngagementID))
	w.line(7, fmt.Sprintf("%s: %s", w.label("generated_at"), data.GeneratedAt.Format("2006-01-02 15:04 UTC")))

	scopeLine := w.label("scope") + ": "
	for i, t := range data.Scope {
		if i > 0 {
			scopeLine += ", "
		}
		scopeLine += t
	}
	w.line(7, scopeLine)
	w.pdf.Ln(6)
}

func (w writer) executiveSummary(data ReportData) {
	w.setFont(14)
	w.line(8, w.label("exec_summary"))
	w.setFont(11)
	w.line(6, w.label("exec_summary_body"))
	w.pdf.Ln(2)

	w.setFont(12)
	w.line(7, w.label("findings_by_sev")+":")
	w.setFont(11)
	summaryLine := fmt.Sprintf("%s: %d   %s: %d   %s: %d   %s: %d   %s: %d",
		w.label("sev_critical"), data.Summary.Critical,
		w.label("sev_high"), data.Summary.High,
		w.label("sev_medium"), data.Summary.Medium,
		w.label("sev_low"), data.Summary.Low,
		w.label("sev_info"), data.Summary.Info,
	)
	w.line(6, summaryLine)
	w.pdf.Ln(6)
}

func (w writer) findings(data ReportData) {
	w.setFont(14)
	w.line(8, w.label("findings"))
	w.pdf.Ln(2)

	if len(data.Findings) == 0 {
		w.setFont(11)
		w.line(6, w.label("no_findings"))
		return
	}

	for i, f := range data.Findings {
		w.setFont(12)
		w.line(7, fmt.Sprintf("%d. %s", i+1, f.Title))

		w.setFont(10)
		rows := [][2]string{
			{w.label("severity"), severityLabel(data.Lang, f.Severity)},
			{w.label("cvss"), fmt.Sprintf("%.1f", f.CVSS)},
			{w.label("owasp"), f.OWASP},
			{w.label("mitre"), f.MITRE},
			{w.label("target"), f.Target},
			{w.label("tool"), f.Tool},
			{w.label("confidence"), string(f.Confidence)},
			{w.label("evidence"), f.Detail},
			{w.label("remediation"), f.Remediation},
		}
		for _, row := range rows {
			w.line(5.5, fmt.Sprintf("%s: %s", row[0], row[1]))
		}
		w.pdf.Ln(4)
	}
}

func (w writer) localStandardsPlaceholder(data ReportData) {
	w.setFont(14)
	w.line(8, w.label("local_standards"))
	w.setFont(11)
	w.line(6, w.label("local_placeholder"))
}
