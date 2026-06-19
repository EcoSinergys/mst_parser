package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

var emptyTVValues = map[string]bool{
	"":    true,
	"-":   true,
	"—":   true,
	"–":   true,
	"n/a": true,
	"na":  true,
}

// ExtractProductTV собирает TV-характеристики товара для MODX.
// Если поле не найдено — оставляет пустую строку и не прерывает парсинг.
func ExtractProductTV(doc *goquery.Document, product *Product) ProductTV {
	tv := ProductTV{
		Category: product.ProductCategory,
	}

	rawSpecs := collectRawTVSpecs(doc, product.Specifications)

	tv.HeadM = findTVValue(rawSpecs, []string{
		"head", "head m", "head meter", "pump head", "max head", "total head",
		"напор", "напор м", "высота напора", "максимальный напор",
		"bosim", "napor", "ko'tarish balandligi", "kotarish balandligi",
	})
	tv.PowerKW = findTVValue(rawSpecs, []string{
		"power", "power kw", "motor power", "engine power", "rated power",
		"мощность", "мощность квт", "мощность двигателя",
		"quvvat", "quvvati", "dvigatel quvvati",
	})
	tv.FlowM3H = findTVValue(rawSpecs, []string{
		"flow", "flow rate", "capacity", "pump capacity", "capacity m3h", "capacity m3/h",
		"расход", "подача", "производительность", "расход м3ч", "расход м3/ч",
		"sarf", "unumdorlik", "oqim", "hajm",
	})
	tv.WeightKG = findTVValue(rawSpecs, []string{
		"weight", "weight kg", "net weight", "gross weight",
		"вес", "масса", "вес кг", "масса кг",
		"vazn", "ogirlik", "og'irlik",
	})
	tv.MaterialBody = findTVValue(rawSpecs, []string{
		"material", "body material", "casing material", "pump casing material", "housing material",
		"материал", "материал корпуса", "корпус", "материал насоса",
		"materiali", "korpus materiali", "nasos materiali",
	})

	logTVExtraction(product.Title, tv)
	return tv
}

// TVToMap преобразует ProductTV в map для JSON-секции "tv".
func TVToMap(tv ProductTV) map[string]string {
	return map[string]string{
		"category":      tv.Category,
		"head_m":        tv.HeadM,
		"power_kw":      tv.PowerKW,
		"flow_m3h":      tv.FlowM3H,
		"weight_kg":     tv.WeightKG,
		"material_body": tv.MaterialBody,
	}
}

func collectRawTVSpecs(doc *goquery.Document, specs map[string]string) map[string]string {
	raw := make(map[string]string)

	for k, v := range specs {
		addRawSpec(raw, k, v)
	}

	doc.Find("table tr").Each(func(i int, row *goquery.Selection) {
		cells := row.Find("td, th")
		if cells.Length() >= 2 {
			key := strings.TrimSpace(cells.First().Text())
			value := strings.TrimSpace(cells.Last().Text())
			addRawSpec(raw, key, value)
		}
	})

	doc.Find("dl").Each(func(i int, dl *goquery.Selection) {
		var lastKey string
		dl.Children().Each(func(j int, item *goquery.Selection) {
			switch goquery.NodeName(item) {
			case "dt":
				lastKey = strings.TrimSpace(item.Text())
			case "dd":
				if lastKey != "" {
					addRawSpec(raw, lastKey, strings.TrimSpace(item.Text()))
					lastKey = ""
				}
			}
		})
	})

	doc.Find("li, p, div, span").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, ":") && len(text) < 250 {
			parts := strings.SplitN(text, ":", 2)
			addRawSpec(raw, parts[0], parts[1])
		}
	})

	return raw
}

func addRawSpec(raw map[string]string, key, value string) {
	key = strings.TrimSpace(strings.TrimRight(key, ":："))
	value = cleanTVValue(value)
	if key == "" || isEmptyTVValue(value) {
		return
	}
	raw[normalizeSpecKey(key)] = value
}

func findTVValue(raw map[string]string, aliases []string) string {
	for _, alias := range aliases {
		normalizedAlias := normalizeSpecKey(alias)
		for key, value := range raw {
			if key == normalizedAlias || strings.Contains(key, normalizedAlias) || strings.Contains(normalizedAlias, key) {
				// Strip unit suffix (e.g., "50 m" -> "50")
				value = strings.TrimSpace(value)
				if strings.Contains(value, " ") {
					value = strings.Split(value, " ")[0]
				}
				return value
			}
		}
	}
	return ""
}

func cleanTVValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, ":： \t\r\n")
	value = strings.Join(strings.Fields(value), " ")
	if isEmptyTVValue(value) {
		return ""
	}
	return value
}

func isEmptyTVValue(value string) bool {
	return emptyTVValues[strings.ToLower(strings.TrimSpace(value))]
}

func normalizeSpecKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "³", "3")
	key = strings.ReplaceAll(key, "㎡", "m2")
	key = strings.ReplaceAll(key, "м³", "м3")
	key = strings.ReplaceAll(key, "м²", "м2")
	key = strings.ReplaceAll(key, "квт", "kw")
	key = strings.ReplaceAll(key, "кг", "kg")

	var b strings.Builder
	prevSpace := false
	for _, r := range key {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSpace = false
			continue
		}
		if !prevSpace {
			b.WriteRune(' ')
			prevSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func logTVExtraction(title string, tv ProductTV) {
	values := TVToMap(tv)
	found := 0
	missing := make([]string, 0)
	for key, value := range values {
		if value == "" {
			missing = append(missing, key)
		} else {
			found++
		}
	}

	fmt.Printf("    TV-характеристик найдено: %d/6\n", found)
	if len(missing) > 0 {
		fmt.Printf("    TV пропущены: %s\n", strings.Join(missing, ", "))
	}
}

var numberWithUnitRe = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*([a-zа-я³/]+)?`)

func extractFirstNumberWithUnit(value string) string {
	match := numberWithUnitRe.FindString(strings.TrimSpace(value))
	if match == "" {
		return value
	}
	return match
}
