package main

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// IsProductImage определяет, является ли изображение "товарным" (нужным для каталога).
// Комбинирует проверку по URL + по контексту в DOM-дереве
func IsProductImage(img *goquery.Selection, pageURL string) bool {
	// Try to get src, if empty try data-zoom-image or data-src
	src, exists := img.Attr("src")
	if !exists || src == "" {
		if zoom, ok := img.Attr("data-zoom-image"); ok && zoom != "" {
			src = zoom
		} else if ds, ok := img.Attr("data-src"); ok && ds != "" {
			src = ds
		} else {
			return false
		}
	}
	imgURL := resolveURL(pageURL, src)

	// 1. Фильтр по URL (иконки, лого, соцсети)
	if isIconOrLogo(imgURL) {
		return false
	}

	// 2. Специальная проверка: если URL изображения содержит slug продукта, считаем его товарным
	slug := strings.TrimSuffix(strings.ToLower(filepath.Base(pageURL)), ".html")
	if strings.Contains(strings.ToLower(imgURL), slug) {
		return true
	}

	// 3. Фильтр по контексту блока
	switch identifyBlock(img) {
	case "MAIN", "DESC":
		return true
	default:
		return false
	}
}

// identifyBlock определяет тип блока, в котором находится изображение
func identifyBlock(img *goquery.Selection) string {
	parents := make([]string, 0)
	img.Parents().Each(func(i int, s *goquery.Selection) {
		if class, ok := s.Attr("class"); ok && class != "" {
			parents = append(parents, class)
		}
		if id, ok := s.Attr("id"); ok && id != "" {
			parents = append(parents, "#"+id)
		}
	})

	parentText := strings.Join(parents, " ")
	parentText = strings.ToLower(parentText)

	src, _ := img.Attr("src")
	lowerSrc := strings.ToLower(src)

	// Пометка как иконка по URL
	for _, p := range []string{"logo", "icon", "favicon", "facebook", "twitter", "linkedin",
		"youtube", "instagram", "whatsapp", "skype", "email", "search", "cart", "basket",
		"arrow", "banner", "arrow", "btn_", "button", "share_", "erweima"} {
		if strings.Contains(lowerSrc, p) {
			return "ICON"
		}
	}

	// Похожие / рекомендованные товары
	if strings.Contains(parentText, "related") ||
		strings.Contains(parentText, "similar") ||
		strings.Contains(parentText, "recommend") {
		return "RELATED"
	}

	// Основная галерея товара
	if strings.Contains(parentText, "gallery") ||
		strings.Contains(parentText, "product-img") ||
		strings.Contains(parentText, "main-image") ||
		strings.Contains(parentText, "product-image") ||
		strings.Contains(parentText, "single-product") ||
		strings.Contains(parentText, "woocommerce-product-gallery") {
		return "MAIN"
	}

	// Миниатюры
	if strings.Contains(parentText, "thumb") ||
		strings.Contains(lowerSrc, "thumb") {
		return "THUMB"
	}

	// В описании / контенте
	if strings.Contains(parentText, "desc") ||
		strings.Contains(parentText, "content") ||
		strings.Contains(parentText, "text") ||
		strings.Contains(parentText, "body") {
		return "DESC"
	}

	return "UNKNOWN"
}

// detectImageContext собирает контекст изображения (родительские теги с классами/id)
func detectImageContext(img *goquery.Selection, pageURL string) string {
	parts := make([]string, 0)

	img.Parents().Each(func(i int, s *goquery.Selection) {
		if i > 3 {
			return
		}
		tag := goquery.NodeName(s)
		class, _ := s.Attr("class")
		id, _ := s.Attr("id")
		ctx := tag
		if class != "" {
			ctx += "." + strings.ReplaceAll(strings.Fields(class)[0], " ", ".")
		}
		if id != "" {
			ctx += "#" + id
		}
		parts = append(parts, ctx)
	})

	if len(parts) > 0 {
		return strings.Join(parts, " > ")
	}
	return "-"
}

// getParentClass возвращает классы ближайшего родителя
func getParentClass(img *goquery.Selection) string {
	parent := img.Parent()
	class, _ := parent.Attr("class")
	tag := goquery.NodeName(parent)
	if class != "" {
		return fmt.Sprintf("%s.%s", tag, strings.Split(class, " ")[0])
	}
	return tag
}

// isIconOrLogo проверяет, является ли изображение иконкой/логотипом по URL
func isIconOrLogo(imgURL string) bool {
	lower := strings.ToLower(imgURL)
	skipPatterns := []string{
		"logo", "icon", "favicon", "banner", "button", "btn_",
		"facebook", "twitter", "linkedin", "youtube", "instagram",
		"whatsapp", "skype", "email", "search", "cart", "basket",
		"arrow", "slide", "thumb_",
		"share_", "lang", "flag", "erweima", "qrcode",
	}
	for _, pattern := range skipPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// resolveURL преобразует относительный URL в абсолютный
func resolveURL(baseURL, relativeURL string) string {
	if strings.HasPrefix(relativeURL, "http") {
		return relativeURL
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return relativeURL
	}
	rel, err := url.Parse(relativeURL)
	if err != nil {
		return relativeURL
	}
	return base.ResolveReference(rel).String()
}

// truncate обрезает строку до maxLen символов
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
