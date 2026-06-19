package main

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Определённая структура категорий (из вашего описания)
var categoryStructure = []CategoryInfo{
	// Slurry Pumps - есть подкатегории
	{Name: "Slurry Pumps", URL: "https://www.mstpumps.com/slurry-pumps/", HasChildren: true},
	// Water Pumps
	{Name: "Water Pumps", URL: "https://www.mstpumps.com/water-pumps/", HasChildren: true},
	// Chemical Pumps
	{Name: "Chemical Pumps", URL: "https://www.mstpumps.com/chemical-pumps/", HasChildren: true},
	// Sewage Pumps
	{Name: "Sewage Pumps", URL: "https://www.mstpumps.com/sewage-pumps/", HasChildren: true},
	// Fire Pumps
	{Name: "Fire Pumps", URL: "https://www.mstpumps.com/fire-pumps/", HasChildren: true},
	// Spare Parts
	{Name: "Spare Parts", URL: "https://www.mstpumps.com/spare-parts/", HasChildren: true},
	// Axial Flow Pumps
	{Name: "Axial Flow Pumps", URL: "https://www.mstpumps.com/axial-flow-pumps/", HasChildren: true},
	// Submersible Pumps - без подкатегорий
	{Name: "Submersible Pumps", URL: "https://www.mstpumps.com/submersible-pumps/", HasChildren: false},
	// Borehole Pump
	{Name: "Borehole Pump", URL: "https://www.mstpumps.com/borehole-pump/", HasChildren: true},
}

// ParsePagination определяет общее количество страниц из элемента пагинации
func ParsePagination(doc *goquery.Document) (int, error) {
	// Ищем элемент с пагинацией — обычно селектор .pagination или similar
	// Ищем текст вида "1/81", "Page 1 of 81"
	var totalPages int

	// Пробуем найти элемент с классом pagination
	doc.Find(".pagination, .pageination, .pager, .pages").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		// Ищем паттерн "X/Y" где X и Y — числа
		parts := strings.Split(text, "/")
		if len(parts) >= 2 {
			lastPart := strings.TrimSpace(parts[len(parts)-1])
			// Извлекаем число из последней части
			fmt.Sscanf(lastPart, "%d", &totalPages)
		}
	})

	// Если не нашли через класс, ищем по тексту "Last"
	if totalPages == 0 {
		doc.Find("a").Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if exists && strings.Contains(href, "page-") {
				// Извлекаем номер последней страницы из ссылки "Last"
				text := strings.TrimSpace(s.Text())
				if strings.ToLower(text) == "last" && href != "" {
					// Парсим номер из href, например /slurry-pumps/page-31/
					parts := strings.Split(strings.TrimRight(href, "/"), "/")
					if len(parts) > 0 {
						lastPart := parts[len(parts)-1]
						lastPart = strings.TrimPrefix(lastPart, "page-")
						fmt.Sscanf(lastPart, "%d", &totalPages)
					}
				}
			}
		})
	}

	return totalPages, nil
}

// GetTotalPagesFromListing определяет количество страниц из текста пагинации
// Ищет текст "First Prev 1 2 3 ... Next Last X/Y"
// Возвращает 0, если пагинация не найдена или если на странице слишком мало продуктов
func GetTotalPagesFromListing(doc *goquery.Document) int {
	totalPages := 0
	linksOnPage := len(ParseProductLinksFromListing(doc))

	// Если на странице меньше 5 продуктов — скорее всего пагинации нет
	if linksOnPage < 5 {
		LogDebug("Мало продуктов на странице (%d), пагинация не требуется", linksOnPage)
		return 0
	}

	// Ищем текст пагинации (может быть в <div> или <span>)
	doc.Find("div, span, p, td").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		// Ищем паттерн "1/81" или "Page 1 of 81"
		if strings.Contains(text, "/") {
			parts := strings.Split(text, "/")
			if len(parts) >= 2 {
				lastPart := strings.TrimSpace(parts[len(parts)-1])
				var n int
				if _, err := fmt.Sscanf(lastPart, "%d", &n); err == nil && n > 0 {
					if n > totalPages && n <= 100 {
						totalPages = n
					}
				}
			}
		}
	})

	// Ограничение: не более 20 страниц
	if totalPages > 20 {
		LogWarn("Пагинация показала %d страниц, ограничиваем до 20", totalPages)
		totalPages = 20
	}

	return totalPages
}

// ParseProductLinksFromListing собирает ссылки на продукты со страницы листинга
func ParseProductLinksFromListing(doc *goquery.Document) []string {
	var links []string
	seen := make(map[string]bool)

	// Ключевые слова для поиска ссылок на продукты
	keywords := []string{
		"Read More", "Read More",
		"read more", "readmore",
		"Подробнее", "подробнее",
		"Batafsil", "batafsil",
		"View Details", "view details",
		"Product Details", "product details",
	}

	// Также собираем все ссылки, заканчивающиеся на .html
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		// Проверяем, заканчивается ли ссылка на .html (страница продукта)
		isHTML := strings.HasSuffix(href, ".html")

		text := strings.TrimSpace(s.Text())
		cleanedText := strings.Join(strings.Fields(text), " ")
		lowerText := strings.ToLower(cleanedText)

		// Проверяем по ключевым словам или по .html
		isProductLink := isHTML
		if !isProductLink {
			for _, keyword := range keywords {
				if strings.EqualFold(cleanedText, keyword) ||
					strings.Contains(lowerText, strings.ToLower(keyword)) {
					isProductLink = true
					break
				}
			}
		}

		if !isProductLink {
			return
		}

		// Пропускаем якорные ссылки, javascript, mailto, tel и навигацию
		if strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") ||
			strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "tel:") {
			return
		}

		// Приводим к абсолютному URL
		if strings.HasPrefix(href, "/") {
			href = "https://www.mstpumps.com" + href
		} else if !strings.HasPrefix(href, "http") {
			href = "https://www.mstpumps.com/" + href
		}

		// Избегаем дубликатов
		if seen[href] {
			return
		}
		seen[href] = true

		links = append(links, href)
	})

	return links
}

// GetSubcategoriesFromSidebar извлекает подкатегории из бокового меню
func GetSubcategoriesFromSidebar(doc *goquery.Document, categoryURL string) []Subcategory {
	var subcategories []Subcategory

	// Ищем боковое меню — возможно <ul class="product-categories">, <div class="sidebar"> и т.д.
	// Пробуем найти ссылки в левой колонке
	doc.Find(".left, .sidebar, .side-menu, .category-menu, .categories").Each(func(i int, s *goquery.Selection) {
		s.Find("a").Each(func(j int, link *goquery.Selection) {
			href, exists := link.Attr("href")
			if exists && href != "" {
				// Приводим к абсолютному URL
				if !strings.HasPrefix(href, "http") {
					href = "https://www.mstpumps.com" + href
				}
				// Проверяем, что ссылка относится к этой категории (начинается с categoryURL)
				if strings.HasPrefix(href, strings.TrimRight(categoryURL, "/")) && href != strings.TrimRight(categoryURL, "/")+"/" {
					name := strings.TrimSpace(link.Text())
					if name != "" {
						subcategories = append(subcategories, Subcategory{
							Name: name,
							URL:  href,
						})
					}
				}
			}
		})
	})

	return subcategories
}

// BuildCategoryURL генерирует URL для страницы категории/подкатегории
func BuildCategoryURL(baseURL string, page int) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if page == 1 {
		return baseURL + "/"
	}
	return fmt.Sprintf("%s/page-%d/", baseURL, page)
}

// IsProductURL проверяет, является ли URL страницей продукта
func IsProductURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	// URL продукта обычно содержит больше сегментов, чем категория
	// и не содержит "/page-"
	for _, part := range parts {
		if strings.HasPrefix(part, "page-") {
			return false
		}
	}
	return len(parts) >= 2
}

// GetProductIDFromURL извлекает идентификатор продукта из URL
func GetProductIDFromURL(productURL string) string {
	parsed, err := url.Parse(productURL)
	if err != nil {
		return ""
	}
	// Берём последний сегмент пути
	base := path.Base(parsed.Path)
	if base == "" || base == "/" {
		return ""
	}
	return strings.TrimSuffix(base, ".html")
}

// GetAliasFromURL создаёт alias для MODX из URL продукта
func GetAliasFromURL(productURL string) string {
	parsed, err := url.Parse(productURL)
	if err != nil {
		return ""
	}
	// Берём последние 2 сегмента или последний сегмент
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return ""
}

// ScrapeCategoryLinks собирает все ссылки на продукты со всех страниц категории/подкатегории
func ScrapeCategoryLinks(sc *ScraperClient, categoryURL string) ([]string, error) {
	var allLinks []string

	LogInfo("Парсинг категории: %s", categoryURL)

	// Начинаем с первой страницы
	resp, err := sc.Get(categoryURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка загрузки категории %s: %v", categoryURL, err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга HTML: %v", err)
	}

	// Собираем ссылки с первой страницы
	links := ParseProductLinksFromListing(doc)
	allLinks = append(allLinks, links...)
	fmt.Printf("  Найдено продуктов на странице 1: %d\n", len(links))

	// Определяем общее количество страниц
	totalPages := GetTotalPagesFromListing(doc)
	fmt.Printf("  Всего страниц в категории: %d\n", totalPages)

	// Если пагинация не определена, пробуем другие методы
	if totalPages == 0 {
		totalPages, err = ParsePagination(doc)
		if err != nil || totalPages == 0 {
			// Если не смогли определить — используем первую страницу
			totalPages = 1
		}
	}

	// Собираем ссылки с остальных страниц
	for page := 2; page <= totalPages; page++ {
		pageURL := BuildCategoryURL(categoryURL, page)
		LogInfo("Страница %d из %d: %s", page, totalPages, pageURL)

		sc.RandomDelay()

		resp, err := sc.Get(pageURL)
		if err != nil {
			LogWarn("Ошибка загрузки страницы %d: %v", page, err)
			// Если 404 — прекращаем обход (страницы закончились)
			if strings.Contains(err.Error(), "404") {
				LogInfo("Страница %d не найдена (404), прекращаем обход", page)
				break
			}
			continue
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		resp.Body.Close()
		if err != nil {
			LogWarn("Ошибка парсинга страницы %d: %v", page, err)
			continue
		}

		links = ParseProductLinksFromListing(doc)
		// Если на странице нет продуктов — прекращаем
		if len(links) == 0 {
			LogInfo("На странице %d нет продуктов, прекращаем обход", page)
			break
		}
		allLinks = append(allLinks, links...)
		LogInfo("Найдено продуктов на странице %d: %d (всего: %d)", page, len(links), len(allLinks))
	}

	return allLinks, nil
}

// ScrapeSubcategoryLinks собирает ссылки на продукты из подкатегории с пагинацией
func ScrapeSubcategoryLinks(sc *ScraperClient, subcategoryURL string) ([]string, error) {
	// Подкатегории используют ту же логику пагинации
	return ScrapeCategoryLinks(sc, subcategoryURL)
}
