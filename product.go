package main

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ParseProductPage парсит страницу продукта и возвращает структуру Product
func ParseProductPage(sc *ScraperClient, productURL string) (*Product, error) {
	fmt.Printf("  🔍 Парсинг продукта: %s\n", productURL)

	resp, err := sc.Get(productURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка загрузки %s: %v", productURL, err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа (может быть в сжатом виде, goquery обработает)
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга HTML: %v", err)
	}

	product := &Product{
		URL:            productURL,
		ProductID:      GetProductIDFromURL(productURL),
		Specifications: make(map[string]string),
	}

	// 1. Заголовок (title) — ищем h1
	doc.Find("h1").Each(func(i int, s *goquery.Selection) {
		if i == 0 {
			product.Title = strings.TrimSpace(s.Text())
		}
	})

	// Если h1 не найден, ищем title в head
	if product.Title == "" {
		product.Title = strings.TrimSpace(doc.Find("title").Text())
	}

	product.Pagetitle = product.Title
	product.Alias = GetAliasFromURL(productURL)

	fmt.Printf("    Название: %s\n", product.Title)

	// 2. Описание — ищем основной контент
	// Пробуем разные селекторы
	descriptionSelectors := []string{
		".product-description",
		"#tab-description",
		".description",
		".product-content",
		".product-detail",
		".content",
		"#content",
		".main-content",
		"article",
	}

	for _, selector := range descriptionSelectors {
		selection := doc.Find(selector)
		if selection.Length() > 0 {
			html, err := selection.Html()
			if err == nil && html != "" {
				product.Description = cleanDescription(html)
				break
			}
		}
	}

	// Если описание не найдено через селекторы, собираем весь контент
	if product.Description == "" {
		product.Description = extractGenericDescription(doc)
	}

	fmt.Printf("    Описание: %d символов\n", len(product.Description))

	// 3. Технические характеристики (Specifications)
	// Ищем таблицу с характеристиками
	doc.Find("table").Each(func(i int, table *goquery.Selection) {
		table.Find("tr").Each(func(j int, row *goquery.Selection) {
			cells := row.Find("td, th")
			if cells.Length() >= 2 {
				key := strings.TrimSpace(cells.First().Text())
				value := strings.TrimSpace(cells.Last().Text())
				if key != "" && value != "" {
					product.Specifications[key] = value
				}
			}
		})
	})

	// Так же ищем характеристики в списках (dt/dd, li)
	doc.Find("ul, dl").Each(func(i int, list *goquery.Selection) {
		list.Find("li, dt").Each(func(j int, item *goquery.Selection) {
			text := strings.TrimSpace(item.Text())
			if strings.Contains(text, ":") {
				parts := strings.SplitN(text, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					if key != "" && value != "" {
						product.Specifications[key] = value
					}
				}
			}
		})
	})

	fmt.Printf("    Характеристик найдено: %d\n", len(product.Specifications))

	// 4. Изображения
	// Маленькие — обычно в галерее, большие — по ссылке или data-zoom-image
	doc.Find("img").Each(func(i int, img *goquery.Selection) {
		src, exists := img.Attr("src")
		if !exists || src == "" {
			return
		}

		// Приводим к абсолютному URL
		imgURL := resolveURL(productURL, src)

		// Пропускаем иконки, логотипы, маленькие картинки
		if isIconOrLogo(imgURL) {
			return
		}

		// Проверяем, есть ли data-zoom-image или data-src для большого изображения
		largeURL := ""
		if dataZoom, exists := img.Attr("data-zoom-image"); exists && dataZoom != "" {
			largeURL = resolveURL(productURL, dataZoom)
		} else if dataSrc, exists := img.Attr("data-src"); exists && dataSrc != "" {
			largeURL = resolveURL(productURL, dataSrc)
		} else if parentLink := img.Parent(); parentLink.Is("a") {
			if href, exists := parentLink.Attr("href"); exists && href != "" {
				largeURL = resolveURL(productURL, href)
			}
		}

		// Если не нашли большое, используем то же что и маленькое
		if largeURL == "" {
			largeURL = imgURL
		}

		imageSet := ImageSet{
			SmallRemoteURL: imgURL,
			LargeRemoteURL: largeURL,
		}

		product.Images = append(product.Images, imageSet)
	})

	fmt.Printf("    Изображений найдено: %d\n", len(product.Images))

	// 5. Product Category — извлекаем из хлебных крошек или URL
	product.ProductCategory = extractCategoryFromBreadcrumbs(doc, productURL)
	if product.ProductCategory == "" {
		product.ProductCategory = extractCategoryFromURL(productURL)
	}

	// 6. Source URL
	product.SourceURL = productURL

	// 7. Template для MODX
	product.Template = 5 // Детальная карточка товара
	product.Published = true

	return product, nil
}

// cleanDescription очищает и форматирует HTML-описание
func cleanDescription(html string) string {
	// Убираем множественные пустые строки
	html = strings.ReplaceAll(html, "<p></p>", "")
	html = strings.ReplaceAll(html, "<p><br/></p>", "")
	html = strings.ReplaceAll(html, "<p><br></p>", "")
	html = strings.TrimSpace(html)
	return html
}

// extractGenericDescription извлекает описание как весь текст body
func extractGenericDescription(doc *goquery.Document) string {
	// Убираем шапку, подвал, меню
	doc.Find("header, footer, nav, script, style, iframe").Remove()

	// Берём основной контент
	body := doc.Find("body")
	body.Find("header, footer, nav, script, style, iframe, .sidebar, .menu, .header, .footer").Remove()

	// Берём HTML основного контента
	html, err := body.Html()
	if err != nil {
		return ""
	}

	// Ограничиваем размер (не больше 50000 символов)
	if len(html) > 50000 {
		html = html[:50000]
	}

	return cleanDescription(html)
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

// isIconOrLogo проверяет, является ли изображение иконкой/логотипом
func isIconOrLogo(imgURL string) bool {
	lower := strings.ToLower(imgURL)
	// Пропускаем очевидные иконки
	skipPatterns := []string{
		"logo", "icon", "favicon", "banner", "button", "btn_",
		"facebook", "twitter", "linkedin", "youtube", "instagram",
		"whatsapp", "skype", "email", "search", "cart", "basket",
		"arrow", "slide", "thumb_",
	}
	for _, pattern := range skipPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// extractCategoryFromBreadcrumbs извлекает название категории из хлебных крошек
func extractCategoryFromBreadcrumbs(doc *goquery.Document, productURL string) string {
	// Ищем хлебные крошки
	breadcrumbSelectors := []string{
		".breadcrumb",
		"#breadcrumb",
		".breadcrumbs",
		".crumbs",
	}

	for _, selector := range breadcrumbSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			s.Find("a").Each(func(j int, link *goquery.Selection) {
				// Последняя ссылка — это сам продукт, предпоследняя — категория
			})
		})
	}

	return ""
}

// extractCategoryFromURL извлекает название категории из URL продукта
func extractCategoryFromURL(productURL string) string {
	parsed, err := url.Parse(productURL)
	if err != nil {
		return ""
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) >= 2 {
		// Категория — предпоследний сегмент
		categoryPart := parts[len(parts)-2]
		// Преобразуем в читаемое название
		categoryPart = strings.ReplaceAll(categoryPart, "-", " ")
		categoryPart = strings.ReplaceAll(categoryPart, "_", " ")
		return strings.Title(categoryPart)
	}

	return ""
}

// CopyAndCloseBody копирует тело ответа и закрывает его (для повторного чтения)
func CopyAndCloseBody(body io.ReadCloser) (io.ReadCloser, error) {
	data, err := io.ReadAll(body)
	body.Close()
	if err != nil {
		return nil, err
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

// ReadBodyAsString читает тело ответа и возвращает как строку
func ReadBodyAsString(body io.ReadCloser) (string, error) {
	data, err := io.ReadAll(body)
	body.Close()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// logProductInfo логирует информацию о продукте
func logProductInfo(product *Product) {
	log.Printf("✅ Продукт: %s", product.Title)
	log.Printf("   URL: %s", product.URL)
	log.Printf("   Изображений: %d", len(product.Images))
	log.Printf("   Характеристик: %d", len(product.Specifications))
}
