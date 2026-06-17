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
	if product.Title == "" {
		product.Title = strings.TrimSpace(doc.Find("title").Text())
	}
	product.Pagetitle = product.Title
	product.Alias = GetAliasFromURL(productURL)
	fmt.Printf("    Название: %s\n", product.Title)

	// 2. Описание — ищем основной контент
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
	if product.Description == "" {
		product.Description = extractGenericDescription(doc)
	}
	fmt.Printf("    Описание: %d символов\n", len(product.Description))

	// 3. Технические характеристики (Specifications)
	// 3a. Из таблиц
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

	// 3b. Из списков (ul/li, dl/dt/dd)
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

	// 3c. Из параграфов с <strong>: <strong>key:</strong> value
	doc.Find("p, div, span").Each(func(i int, s *goquery.Selection) {
		s.Find("strong, b").Each(func(j int, strong *goquery.Selection) {
			text := strings.TrimSpace(strong.Text())
			if !strings.HasSuffix(text, ":") && !strings.Contains(text, ":") {
				return
			}
			// Берём текст родителя после <strong>
			parentHtml, err := strong.Parent().Html()
			if err != nil {
				return
			}
			cleanKey := strings.TrimRight(strings.TrimSpace(text), ":")
			// Извлекаем значение после </strong>
			strongHtml, err := goquery.OuterHtml(strong)
			if err != nil {
				return
			}
			parts := strings.SplitN(parentHtml, strongHtml, 2)
			if len(parts) == 2 {
				value := strings.TrimSpace(stripHTML(parts[1]))
				// Убираем точку с запятой, запятые в конце
				value = strings.TrimRight(value, ".,; ")
				if cleanKey != "" && value != "" && len(value) < 200 {
					product.Specifications[cleanKey] = value
				}
			}
		})
	})

	fmt.Printf("    Характеристик найдено: %d\n", len(product.Specifications))

	// 4. Изображения — собираем только товарные (фильтр по URL + контексту)
	doc.Find("img").Each(func(i int, img *goquery.Selection) {
		src, exists := img.Attr("src")
		if !exists || src == "" {
			return
		}

		// Пропускаем, если изображение не товарное
		if !IsProductImage(img, productURL) {
			return
		}

		imgURL := resolveURL(productURL, src)
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
		if largeURL == "" {
			largeURL = imgURL
		}
		product.Images = append(product.Images, ImageSet{
			SmallRemoteURL: imgURL,
			LargeRemoteURL: largeURL,
		})
	})
	fmt.Printf("    Изображений найдено: %d\n", len(product.Images))

	// 5. Категория
	product.ProductCategory = extractCategoryFromBreadcrumbs(doc, productURL)
	if product.ProductCategory == "" {
		product.ProductCategory = extractCategoryFromURL(productURL)
	}

	// 6. TV-характеристики (должны быть ПОСЛЕ заполнения Specifications)
	product.TV = ExtractProductTV(doc, product)

	product.SourceURL = productURL
	product.Template = 5
	product.Published = true

	return product, nil
}

// cleanDescription очищает и форматирует HTML-описание
func cleanDescription(html string) string {
	html = strings.ReplaceAll(html, "<p></p>", "")
	html = strings.ReplaceAll(html, "<p><br/></p>", "")
	html = strings.ReplaceAll(html, "<p><br></p>", "")
	return strings.TrimSpace(html)
}

// extractGenericDescription извлекает описание как весь текст body
func extractGenericDescription(doc *goquery.Document) string {
	doc.Find("header, footer, nav, script, style, iframe").Remove()
	body := doc.Find("body")
	body.Find("header, footer, nav, script, style, iframe, .sidebar, .menu, .header, .footer").Remove()
	html, err := body.Html()
	if err != nil {
		return ""
	}
	if len(html) > 50000 {
		html = html[:50000]
	}
	return cleanDescription(html)
}

// extractCategoryFromBreadcrumbs извлекает название категории из хлебных крошек
func extractCategoryFromBreadcrumbs(doc *goquery.Document, productURL string) string {
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
		categoryPart := parts[len(parts)-2]
		categoryPart = strings.ReplaceAll(categoryPart, "-", " ")
		categoryPart = strings.ReplaceAll(categoryPart, "_", " ")
		return strings.Title(categoryPart)
	}
	return ""
}

// stripHTML удаляет HTML-теги из строки
func stripHTML(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return html
	}
	return strings.TrimSpace(doc.Text())
}

// CopyAndCloseBody копирует тело ответа и закрывает его
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
