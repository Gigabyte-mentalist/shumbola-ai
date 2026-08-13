package report

import "github.com/shumbola/agent-core/internal/tools"

// Lang identifiers match the web-ui's i18next locale codes (TZ 6.5).
const (
	LangUzLatn = "uz-Latn"
	LangUzCyrl = "uz-Cyrl"
	LangRu     = "ru"
)

// labels holds the report's static text in each supported language.
// Keep this in sync with the label keys used by pdf.go.
var labels = map[string]map[string]string{
	LangUzLatn: {
		"title":             "shumbola.ai — Pentest hisoboti",
		"engagement":        "Engagement",
		"generated_at":      "Yaratilgan sana",
		"scope":             "Doira (scope)",
		"exec_summary":      "Boshqaruv xulosasi",
		"exec_summary_body": "Ushbu hisobot avtomatik ravishda generatsiya qilingan. Faqat buzmasdan tasdiqlangan (proof-of-exploit) topilmalar keltirilgan.",
		"findings_by_sev":   "Jiddiylik bo'yicha topilmalar",
		"findings":          "Topilmalar",
		"no_findings":       "Tasdiqlangan topilma aniqlanmadi.",
		"severity":          "Jiddiylik",
		"cvss":              "CVSS (taxminiy)",
		"owasp":             "OWASP Top 10",
		"mitre":             "MITRE ATT&CK",
		"target":            "Nishon",
		"tool":              "Tool",
		"confidence":        "Ishonch darajasi",
		"evidence":          "Dalil",
		"remediation":       "Tuzatish tavsiyasi",
		"local_standards":   "Mahalliy talablar (O'zbekiston DXX/O'zDSt)",
		"local_placeholder": "Ushbu bo'lim keyingi bosqichda to'ldiriladi.",
		"sev_critical":      "Kritik",
		"sev_high":          "Yuqori",
		"sev_medium":        "O'rta",
		"sev_low":           "Past",
		"sev_info":          "Ma'lumot",
	},
	LangUzCyrl: {
		"title":             "shumbola.ai — Пентест ҳисоботи",
		"engagement":        "Энгейджмент",
		"generated_at":      "Яратилган сана",
		"scope":             "Доира (scope)",
		"exec_summary":      "Бошқарув хулосаси",
		"exec_summary_body": "Ушбу ҳисобот автоматик равишда генерация қилинган. Фақат бузмасдан тасдиқланган (proof-of-exploit) топилмалар келтирилган.",
		"findings_by_sev":   "Жиддийлик бўйича топилмалар",
		"findings":          "Топилмалар",
		"no_findings":       "Тасдиқланган топилма аниқланмади.",
		"severity":          "Жиддийлик",
		"cvss":              "CVSS (тахминий)",
		"owasp":             "OWASP Top 10",
		"mitre":             "MITRE ATT&CK",
		"target":            "Нишон",
		"tool":              "Tool",
		"confidence":        "Ишонч даражаси",
		"evidence":          "Далил",
		"remediation":       "Тузатиш тавсияси",
		"local_standards":   "Маҳаллий талаблар (Ўзбекистон DXX/ЎзДСт)",
		"local_placeholder": "Ушбу бўлим кейинги босқичда тўлдирилади.",
		"sev_critical":      "Критик",
		"sev_high":          "Юқори",
		"sev_medium":        "Ўрта",
		"sev_low":           "Паст",
		"sev_info":          "Маълумот",
	},
	LangRu: {
		"title":             "shumbola.ai — Отчёт по пентесту",
		"engagement":        "Проект (engagement)",
		"generated_at":      "Дата создания",
		"scope":             "Область (scope)",
		"exec_summary":      "Резюме для руководства",
		"exec_summary_body": "Данный отчёт сгенерирован автоматически. Указаны только подтверждённые (proof-of-exploit) находки.",
		"findings_by_sev":   "Находки по уровню серьёзности",
		"findings":          "Находки",
		"no_findings":       "Подтверждённых находок не обнаружено.",
		"severity":          "Серьёзность",
		"cvss":              "CVSS (приблизительно)",
		"owasp":             "OWASP Top 10",
		"mitre":             "MITRE ATT&CK",
		"target":            "Цель",
		"tool":              "Инструмент",
		"confidence":        "Уровень доверия",
		"evidence":          "Доказательство",
		"remediation":       "Рекомендация по устранению",
		"local_standards":   "Локальные требования (Узбекистан DXX/O'zDSt)",
		"local_placeholder": "Этот раздел будет заполнен на следующем этапе.",
		"sev_critical":      "Критическая",
		"sev_high":          "Высокая",
		"sev_medium":        "Средняя",
		"sev_low":           "Низкая",
		"sev_info":          "Информационная",
	},
}

// label returns the localized string for key in lang, falling back to
// LangUzLatn if lang is unrecognized, and to the key itself if the label
// is somehow missing (should never happen given the maps above).
func label(lang, key string) string {
	set, ok := labels[lang]
	if !ok {
		set = labels[LangUzLatn]
	}
	if v, ok := set[key]; ok {
		return v
	}
	return key
}

func severityLabel(lang string, sev tools.Severity) string {
	switch string(sev) {
	case "critical":
		return label(lang, "sev_critical")
	case "high":
		return label(lang, "sev_high")
	case "medium":
		return label(lang, "sev_medium")
	case "low":
		return label(lang, "sev_low")
	default:
		return label(lang, "sev_info")
	}
}
