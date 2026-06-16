package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func main() {
	mode := flag.String("mode", "B", "Режим парсинга: B - по категориям, A - верификация через productlist")
	categoryURL := flag.String("category", "", "URL конкретной категории для парсинга (опционально)")
	skipImages := flag.Bool("skip-images", false, "Пропустить скачивание изображений")
	limit := flag.Int("limit", 0, "Ограничить количество продуктов (0 = без лимита)")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║    MST Pumps Catalog Parser v2.0            ║")
	fmt.Println("║    Парсер каталога насосов MST Pumps        ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Printf("Режим: %s\n", *mode)
	if *skipImages {
		fmt.Println("Скачивание изображений: отключено")
	}
	if *limit > 0 {
		fmt.Printf("Лимит продуктов: %d\n", *limit)
	}

	sc := NewScraperClient(1500*time.Millisecond, 4000*time.Millisecond, 5)

	downloader := NewDownloader("downloaded_images")
	if !*skipImages {
		if err := downloader.InitDirs(); err != nil {
			log.Fatalf("❌ Ошибка создания директорий: %v", err)
		}
	}

	catalog := &Catalog{}

	switch *mode {
	case "B":
		if *categoryURL != "" {
			parseSingleCategory(sc, downloader, catalog, *categoryURL, *skipImages, *limit)
		} else {
			parseAllCategories(sc, downloader, catalog, *skipImages, *limit)
		}
	case "A":
		fmt.Println("\n=== Режим верификации A: сбор всех продуктов через productlist ===")
		parseProductList(sc, downloader, catalog, *skipImages, *limit)
	default:
		log.Fatalf("❌ Неизвестный режим: %s. Используйте A или B", *mode)
	}

	if err := SaveCatalog(catalog, "catalog_structured.json"); err != nil {
		log.Printf("⚠️ Ошибка сохранения каталога: %v", err)
	}
	modxProducts := ConvertCatalogToMODXProducts(catalog)
	if err := SaveMODXImport(modxProducts, "modx_import.json"); err != nil {
		log.Printf("⚠️ Ошибка сохранения MODX-импорта: %v", err)
	}
	PrintSummary(catalog)
	fmt.Println("\n🎉 Парсинг завершён!")
}

// -------------------- Парсинг всех категорий (режим B) --------------------

func parseAllCategories(sc *ScraperClient, downloader *Downloader, catalog *Catalog, skipImages bool, limit int) {
	fmt.Println("\n=== Режим B: Парсинг по дереву категорий ===")
	globalProductCount := 0

	for _, catInfo := range categoryStructure {
		fmt.Printf("\n📁 Категория: %s (%s)\n", catInfo.Name, catInfo.URL)
		category := Category{Name: catInfo.Name, URL: catInfo.URL}

		if catInfo.HasChildren {
			_, err := sc.Get(catInfo.URL)
			if err != nil {
				log.Printf("⚠️ Ошибка загрузки категории %s: %v", catInfo.URL, err)
			}
			subcategories := getPredefinedSubcategories(catInfo.Name)
			if len(subcategories) == 0 {
				fmt.Printf("  ⚠️ Нет подкатегорий для %s\n", catInfo.Name)
				continue
			}
			for _, sub := range subcategories {
				fmt.Printf("\n  �� Подкатегория: %s (%s)\n", sub.Name, sub.URL)
				links, err := ScrapeSubcategoryLinks(sc, sub.URL)
				if err != nil {
					log.Printf("  ⚠️ Ошибка парсинга подкатегории %s: %v", sub.URL, err)
					continue
				}
				fmt.Printf("  Найдено продуктов: %d\n", len(links))
				var products []Product
				for i, link := range links {
					if limit > 0 && globalProductCount >= limit {
						fmt.Println("  Достигнут лимит продуктов")
						break
					}
					product, err := ParseProductPage(sc, link)
					if err != nil {
						log.Printf("  ⚠️ Ошибка парсинга продукта %s: %v", link, err)
						continue
					}
					if !skipImages {
						downloader.DownloadProductImages(product)
					}
					products = append(products, *product)
					globalProductCount++
					if i < len(links)-1 {
						sc.RandomDelay()
					}
				}
				sub.Products = products
				category.Subcategories = append(category.Subcategories, sub)
			}
		} else {
			fmt.Printf("  Категория без подкатегорий, парсинг продуктов напрямую...\n")
			links, err := ScrapeCategoryLinks(sc, catInfo.URL)
			if err != nil {
				log.Printf("  ⚠️ Ошибка парсинга категории %s: %v", catInfo.URL, err)
				continue
			}
			var products []Product
			for i, link := range links {
				if limit > 0 && globalProductCount >= limit {
					break
				}
				product, err := ParseProductPage(sc, link)
				if err != nil {
					log.Printf("  ⚠️ Ошибка парсинга продукта %s: %v", link, err)
					continue
				}
				if !skipImages {
					downloader.DownloadProductImages(product)
				}
				products = append(products, *product)
				globalProductCount++
				if i < len(links)-1 {
					sc.RandomDelay()
				}
			}
			category.Subcategories = append(category.Subcategories, Subcategory{Name: catInfo.Name, URL: catInfo.URL, Products: products})
		}
		catalog.Categories = append(catalog.Categories, category)
	}
}

func parseSingleCategory(sc *ScraperClient, downloader *Downloader, catalog *Catalog, categoryURL string, skipImages bool, limit int) {
	fmt.Printf("\n=== Парсинг одной категории: %s ===\n", categoryURL)
	catInfo := CategoryInfo{Name: extractCategoryNameFromURL(categoryURL), URL: categoryURL, HasChildren: true}
	category := Category{Name: catInfo.Name, URL: catInfo.URL}
	subcategories := getPredefinedSubcategories(catInfo.Name)

	if len(subcategories) == 0 {
		links, err := ScrapeCategoryLinks(sc, categoryURL)
		if err != nil {
			log.Printf("⚠️ Ошибка: %v", err)
			return
		}
		var products []Product
		for i, link := range links {
			if limit > 0 && i >= limit {
				break
			}
			product, err := ParseProductPage(sc, link)
			if err != nil {
				continue
			}
			if !skipImages {
				downloader.DownloadProductImages(product)
			}
			products = append(products, *product)
			sc.RandomDelay()
		}
		category.Subcategories = append(category.Subcategories, Subcategory{Name: catInfo.Name, URL: categoryURL, Products: products})
	} else {
		for _, sub := range subcategories {
			links, err := ScrapeSubcategoryLinks(sc, sub.URL)
			if err != nil {
				continue
			}
			var products []Product
			for i, link := range links {
				if limit > 0 && i >= limit {
					break
				}
				product, err := ParseProductPage(sc, link)
				if err != nil {
					continue
				}
				if !skipImages {
					downloader.DownloadProductImages(product)
				}
				products = append(products, *product)
				sc.RandomDelay()
			}
			sub.Products = products
			category.Subcategories = append(category.Subcategories, sub)
		}
	}
	catalog.Categories = append(catalog.Categories, category)
}

// -------------------- Парсинг через productlist (режим A) с кэшом --------------------

// loadLinksCache загружает кэш ссылок из файла
func loadLinksCache(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var links []string
	if err := json.Unmarshal(data, &links); err != nil {
		return nil, err
	}
	return links, nil
}

// saveLinksCache сохраняет ссылки в кэш
func saveLinksCache(links []string, filePath string) error {
	data, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// loadProductCache загружает уже спарсенные продукты
func loadProductCache(filePath string) (map[string]bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return make(map[string]bool), nil // если файла нет — пустой кэш
	}
	var urls []string
	if err := json.Unmarshal(data, &urls); err != nil {
		return make(map[string]bool), nil
	}
	cache := make(map[string]bool, len(urls))
	for _, u := range urls {
		cache[u] = true
	}
	return cache, nil
}

// saveProductCache сохраняет URL спарсенных продуктов
func saveProductCache(urls []string, filePath string) error {
	data, err := json.MarshalIndent(urls, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

func parseProductList(sc *ScraperClient, downloader *Downloader, catalog *Catalog, skipImages bool, limit int) {
	const linksCacheFile = "_product_links.json"
	const productCacheFile = "_product_cache.json"

	// 1. Загружаем кэш ссылок на продукты
	allLinks, err := loadLinksCache(linksCacheFile)
	if err != nil || len(allLinks) == 0 {
		fmt.Println("Сбор всех ссылок на продукты с productlist (1-81)...")
		allLinks = nil
		for page := 1; page <= 81; page++ {
			var url string
			if page == 1 {
				url = "https://www.mstpumps.com/products"
			} else {
				url = fmt.Sprintf("https://www.mstpumps.com/productlist-%d", page)
			}
			fmt.Printf("Страница %d/81: %s\n", page, url)
			var links []string
			for retry := 0; retry < 3; retry++ {
				if retry > 0 {
					fmt.Printf("  🔄 Повторная попытка %d...\n", retry+1)
					sc.RandomDelay()
					sc.RandomDelay()
				}
				resp, err := sc.Get(url)
				if err != nil {
					log.Printf("⚠️ Ошибка: %v", err)
					continue
				}
				doc, err := goquery.NewDocumentFromReader(resp.Body)
				resp.Body.Close()
				if err != nil {
					log.Printf("⚠️ Ошибка парсинга: %v", err)
					continue
				}
				links = ParseProductLinksFromListing(doc)
				if len(links) > 0 || retry >= 2 {
					break
				}
				fmt.Printf("  ⚠️ Получено 0 товаров (попытка %d), повторяю...\n", retry+1)
			}
			allLinks = append(allLinks, links...)
			fmt.Printf("  Найдено: %d (всего: %d)\n", len(links), len(allLinks))
			if page < 81 {
				sc.RandomDelay()
			}
		}
		// Сохраняем кэш ссылок
		if err := saveLinksCache(allLinks, linksCacheFile); err != nil {
			log.Printf("⚠️ Ошибка сохранения кэша ссылок: %v", err)
		} else {
			fmt.Printf("💾 Ссылки сохранены в кэш: %s (%d ссылок)\n", linksCacheFile, len(allLinks))
		}
		fmt.Printf("\nВсего собрано ссылок: %d\n", len(allLinks))
	} else {
		fmt.Printf("📦 Загружено %d ссылок из кэша (%s)\n", len(allLinks), linksCacheFile)
	}

	// 2. Загружаем кэш уже спарсенных продуктов
	parsedCache, err := loadProductCache(productCacheFile)
	if err != nil {
		parsedCache = make(map[string]bool)
	}
	fmt.Printf("📦 Ранее спарсено: %d продуктов\n", len(parsedCache))

	// 3. Парсим только НОВЫЕ продукты
	category := Category{Name: "All Products", URL: "https://www.mstpumps.com/products"}
	var products []Product
	var newlyParsed []string
	doneCount := len(parsedCache)

	for i, link := range allLinks {
		if limit > 0 && len(products) >= limit {
			break
		}
		// Пропускаем уже спарсенное
		if parsedCache[link] {
			// Восстанавливаем продукт из ранее собранного (без повторного парсинга)
			// Создаём минимальную запись
			fmt.Printf("⏭️ Продукт %d/%d (пропущен, уже есть в кэше)\n", i+1, len(allLinks))
			continue
		}

		product, err := ParseProductPage(sc, link)
		if err != nil {
			log.Printf("⚠️ Ошибка: %v", err)
			continue
		}
		if !skipImages {
			downloader.DownloadProductImages(product)
		}
		products = append(products, *product)
		newlyParsed = append(newlyParsed, link)
		fmt.Printf("✅ Продукт %d/%d: %s\n", doneCount+len(newlyParsed), len(allLinks), product.Title)
		if i < len(allLinks)-1 {
			sc.RandomDelay()
		}

		// Сохраняем кэш после каждых 10 новых продуктов (на случай прерывания)
		if len(newlyParsed)%10 == 0 && len(newlyParsed) > 0 {
			allParsed := make([]string, 0, len(parsedCache)+len(newlyParsed))
			for k := range parsedCache {
				allParsed = append(allParsed, k)
			}
			allParsed = append(allParsed, newlyParsed...)
			saveProductCache(allParsed, productCacheFile)
		}
	}

	// Сохраняем финальный кэш
	if len(newlyParsed) > 0 {
		allParsed := make([]string, 0, len(parsedCache)+len(newlyParsed))
		for k := range parsedCache {
			allParsed = append(allParsed, k)
		}
		allParsed = append(allParsed, newlyParsed...)
		saveProductCache(allParsed, productCacheFile)
		fmt.Printf("💾 Кэш продуктов сохранён: %s (%d продуктов)\n", productCacheFile, len(allParsed))
	}

	category.Subcategories = append(category.Subcategories, Subcategory{Name: "All Products", URL: "https://www.mstpumps.com/products", Products: products})
	catalog.Categories = append(catalog.Categories, category)
}

// -------------------- Подкатегории и утилиты --------------------

func getPredefinedSubcategories(categoryName string) []Subcategory {
	subcategoryMap := map[string][]Subcategory{
		"Slurry Pumps": {
			{Name: "Heavy Duty Slurry Pump", URL: "https://www.mstpumps.com/slurry-pumps/heavy-duty-slurry-pump/"},
			{Name: "Sump Slurry Pump", URL: "https://www.mstpumps.com/slurry-pumps/sump-slurry-pump/"},
			{Name: "Gravel Sand Pump", URL: "https://www.mstpumps.com/slurry-pumps/gravel-sand-pump/"},
			{Name: "Submersible Slurry Pump", URL: "https://www.mstpumps.com/slurry-pumps/submersible-slurry-pump/"},
			{Name: "Froth Pump", URL: "https://www.mstpumps.com/slurry-pumps/froth-pump/"},
		},
		"Water Pumps": {
			{Name: "End Suction Water Pump", URL: "https://www.mstpumps.com/water-pumps/end-suction-water-pump/"},
			{Name: "Split Casing Water Pump", URL: "https://www.mstpumps.com/water-pumps/split-casing-water-pump/"},
			{Name: "Multistage Water Pump", URL: "https://www.mstpumps.com/water-pumps/multistage-water-pump/"},
			{Name: "Diesel Water Pump", URL: "https://www.mstpumps.com/water-pumps/diesel-water-pump/"},
		},
		"Chemical Pumps": {
			{Name: "Stainless Steel Chemical Pump", URL: "https://www.mstpumps.com/chemical-pumps/stainless-steel-chemical-pump/"},
			{Name: "Fluoroplastic Chemical Pump", URL: "https://www.mstpumps.com/chemical-pumps/fluoroplastic-chemical-pump/"},
		},
		"Sewage Pumps": {
			{Name: "Submersible Sewage Pump", URL: "https://www.mstpumps.com/sewage-pumps/submersible-sewage-pump/"},
			{Name: "Self-Priming Sewage Pump", URL: "https://www.mstpumps.com/sewage-pumps/self-priming-sewage-pump/"},
		},
		"Fire Pumps": {
			{Name: "Electric Fire Pump", URL: "https://www.mstpumps.com/fire-pumps/electric-fire-pump/"},
			{Name: "Diesel Fire Pump", URL: "https://www.mstpumps.com/fire-pumps/diesel-fire-pump/"},
		},
		"Spare Parts": {
			{Name: "Slurry Pump Spare Parts", URL: "https://www.mstpumps.com/spare-parts/slurry-pump-spare-parts/"},
			{Name: "Water Pump Spare Parts", URL: "https://www.mstpumps.com/spare-parts/water-pump-spare-parts/"},
			{Name: "OEM Spare Parts", URL: "https://www.mstpumps.com/spare-parts/oem-spare-parts/"},
		},
		"Axial Flow Pumps": {
			{Name: "Submersible Axial Flow Pumps", URL: "https://www.mstpumps.com/axial-flow-pumps/submersible-axial-flow-pumps/"},
			{Name: "Vertical Axial Flow Pumps", URL: "https://www.mstpumps.com/axial-flow-pumps/vertical-axial-flow-pumps/"},
		},
		"Submersible Pumps": {},
		"Borehole Pump": {
			{Name: "Stainless Steel Borehole Pump", URL: "https://www.mstpumps.com/borehole-pump/stainless-steel-borehole-pump/"},
			{Name: "Cast Iron Borehole Pump", URL: "https://www.mstpumps.com/borehole-pump/cast-iron-borehole-pump/"},
		},
	}
	return subcategoryMap[categoryName]
}

func extractCategoryNameFromURL(rawURL string) string {
	parts := strings.Split(strings.TrimRight(rawURL, "/"), "/")
	if len(parts) > 0 {
		name := strings.ReplaceAll(parts[len(parts)-1], "-", " ")
		name = strings.ReplaceAll(name, "_", " ")
		return strings.Title(name)
	}
	return "Unknown"
}
