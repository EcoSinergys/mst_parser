package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

// parseDirectProducts парсит недостающие товары из sitemap_urls.txt
func parseDirectProducts(sc *ScraperClient) {
	fmt.Println("\n=== Режим direct: допарсинг недостающих товаров из sitemap ===")

	// 1. Читаем sitemap_urls.txt
	sitemapData, err := os.ReadFile("sitemap_urls.txt")
	if err != nil {
		log.Fatalf("❌ Ошибка чтения sitemap_urls.txt: %v", err)
	}
	sitemapLines := strings.Split(strings.TrimSpace(string(sitemapData)), "\n")
	fmt.Printf("📄 Загружено URL из sitemap: %d\n", len(sitemapLines))

	// 2. Фильтруем только товарные URL
	var productURLs []string
	for _, u := range sitemapLines {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if !strings.HasSuffix(u, ".html") {
			continue
		}
		if strings.Contains(u, "/news/") || strings.Contains(u, "/info/") {
			continue
		}
		productURLs = append(productURLs, u)
	}
	fmt.Printf("📦 Товарных URL: %d\n", len(productURLs))

	// 3. Загружаем существующий каталог
	existingCatalog := &Catalog{}
	parsedURLs := make(map[string]bool)
	if data, err := os.ReadFile("catalog_structured.json"); err == nil {
		if err := json.Unmarshal(data, existingCatalog); err != nil {
			log.Printf("⚠️ Ошибка парсинга catalog_structured.json: %v", err)
			existingCatalog = &Catalog{}
		}
	}
	for _, cat := range existingCatalog.Categories {
		for _, sub := range cat.Subcategories {
			for _, prod := range sub.Products {
				parsedURLs[prod.URL] = true
			}
		}
	}
	fmt.Printf("✅ Уже спарсено: %d\n", len(parsedURLs))

	// 4. Находим недостающие URL
	var missingURLs []string
	for _, u := range productURLs {
		if !parsedURLs[u] {
			missingURLs = append(missingURLs, u)
		}
	}
	fmt.Printf("🆕 Недостающих товаров: %d\n", len(missingURLs))

	if len(missingURLs) == 0 {
		fmt.Println("✅ Все товары уже спарсены!")
		return
	}

	// 5. Парсим недостающие товары
	totalParsed := 0
	totalErrors := 0
	for i, u := range missingURLs {
		fmt.Printf("  [%d/%d] %s\n", i+1, len(missingURLs), u)
		product, err := ParseProductPage(sc, u)
		if err != nil {
			log.Printf("  ⚠️ Ошибка парсинга: %v", err)
			totalErrors++
			continue
		}
		// Добавляем в каталог
		existingCatalog.Categories = append(existingCatalog.Categories, Category{
			Name: "Direct Products",
			URL:  "https://www.mstpumps.com/",
			Subcategories: []Subcategory{{
				Name: "Direct Products",
				URL:  "https://www.mstpumps.com/",
				Products: []Product{*product},
			}},
		})
		totalParsed++
		fmt.Printf("  ✅ %s\n", product.Title)
		if i < len(missingURLs)-1 {
			sc.RandomDelay()
		}
	}

	// 6. Сохраняем
	EnrichCatalogWithDestPaths(existingCatalog)
	SaveCatalog(existingCatalog, "catalog_structured.json")
	modxProducts := ConvertCatalogToMODXProducts(existingCatalog)
	SaveMODXImport(modxProducts, "modx_import.json")
	PrintSummary(existingCatalog)

	fmt.Printf("\n✅ Успешно спарсено: %d\n", totalParsed)
	fmt.Printf("❌ Ошибок: %d\n", totalErrors)
}