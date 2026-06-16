package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func main() {
	// Параметры командной строки
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

	// Создаём клиент с защитой
	sc := NewScraperClient(1500*time.Millisecond, 4000*time.Millisecond, 5)

	// Создаём загрузчик изображений
	downloader := NewDownloader("downloaded_images")
	if !*skipImages {
		if err := downloader.InitDirs(); err != nil {
			log.Fatalf("❌ Ошибка создания директорий: %v", err)
		}
	}

	// Инициализируем каталог
	catalog := &Catalog{}

	switch *mode {
	case "B":
		// Режим B — обход по категориям
		if *categoryURL != "" {
			// Парсинг одной категории
			parseSingleCategory(sc, downloader, catalog, *categoryURL, *skipImages, *limit)
		} else {
			// Парсинг всех категорий
			parseAllCategories(sc, downloader, catalog, *skipImages, *limit)
		}

	case "A":
		// Режим A — верификация через productlist
		fmt.Println("\n=== Режим верификации A: сбор всех продуктов через productlist ===")
		parseProductList(sc, downloader, catalog, *skipImages, *limit)

	default:
		log.Fatalf("❌ Неизвестный режим: %s. Используйте A или B", *mode)
	}

	// Сохраняем структурированный каталог
	if err := SaveCatalog(catalog, "catalog_structured.json"); err != nil {
		log.Printf("⚠️ Ошибка сохранения каталога: %v", err)
	}

	// Конвертируем и сохраняем для MODX
	modxProducts := ConvertCatalogToMODXProducts(catalog)
	if err := SaveMODXImport(modxProducts, "modx_import.json"); err != nil {
		log.Printf("⚠️ Ошибка сохранения MODX-импорта: %v", err)
	}

	// Выводим сводку
	PrintSummary(catalog)
	fmt.Println("\n🎉 Парсинг завершён!")
}

// parseAllCategories парсит все категории из предопределённой структуры
func parseAllCategories(sc *ScraperClient, downloader *Downloader, catalog *Catalog, skipImages bool, limit int) {
	fmt.Println("\n=== Режим B: Парсинг по дереву категорий ===")

	globalProductCount := 0

	for _, catInfo := range categoryStructure {
		fmt.Printf("\n📁 Категория: %s (%s)\n", catInfo.Name, catInfo.URL)

		category := Category{
			Name: catInfo.Name,
			URL:  catInfo.URL,
		}

		if catInfo.HasChildren {
			fmt.Printf("  Загрузка страницы категории для получения подкатегорий...\n")
			_, err := sc.Get(catInfo.URL)
			if err != nil {
				log.Printf("⚠️ Ошибка загрузки категории %s: %v", catInfo.URL, err)
			}

			// Используем предопределённые подкатегории для этой категории
			subcategories := getPredefinedSubcategories(catInfo.Name)

			if len(subcategories) == 0 {
				fmt.Printf("  ⚠️ Нет подкатегорий для %s\n", catInfo.Name)
				continue
			}

			for _, sub := range subcategories {
				fmt.Printf("\n  📂 Подкатегория: %s (%s)\n", sub.Name, sub.URL)

				// Получаем ссылки на продукты
				links, err := ScrapeSubcategoryLinks(sc, sub.URL)
				if err != nil {
					log.Printf("  ⚠️ Ошибка парсинга подкатегории %s: %v", sub.URL, err)
					continue
				}

				fmt.Printf("  Найдено продуктов: %d\n", len(links))

				// Парсим каждый продукт
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

					// Скачиваем изображения
					if !skipImages {
						if err := downloader.DownloadProductImages(product); err != nil {
							log.Printf("  ⚠️ Ошибка скачивания изображений: %v", err)
						}
					}

					products = append(products, *product)
					globalProductCount++

					// Пауза между продуктами
					if i < len(links)-1 {
						sc.RandomDelay()
					}
				}

				// Обновляем подкатегорию с продуктами
				sub.Products = products
				category.Subcategories = append(category.Subcategories, sub)
			}
		} else {
			// Категория без подкатегорий — парсим продукты напрямую
			fmt.Printf("  Категория без подкатегорий, парсинг продуктов напрямую...\n")
			links, err := ScrapeCategoryLinks(sc, catInfo.URL)
			if err != nil {
				log.Printf("  ⚠️ Ошибка парсинга категории %s: %v", catInfo.URL, err)
				continue
			}

			// Создаём псевдо-подкатегорию с тем же именем
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
					if err := downloader.DownloadProductImages(product); err != nil {
						log.Printf("  ⚠️ Ошибка скачивания изображений: %v", err)
					}
				}

				products = append(products, *product)
				globalProductCount++

				if i < len(links)-1 {
					sc.RandomDelay()
				}
			}

			category.Subcategories = append(category.Subcategories, Subcategory{
				Name:     catInfo.Name,
				URL:      catInfo.URL,
				Products: products,
			})
		}

		catalog.Categories = append(catalog.Categories, category)
	}
}

// parseSingleCategory парсит одну категорию по URL
func parseSingleCategory(sc *ScraperClient, downloader *Downloader, catalog *Catalog, categoryURL string, skipImages bool, limit int) {
	fmt.Printf("\n=== Парсинг одной категории: %s ===\n", categoryURL)

	catInfo := CategoryInfo{
		Name:        extractCategoryNameFromURL(categoryURL),
		URL:         categoryURL,
		HasChildren: true,
	}

	category := Category{
		Name: catInfo.Name,
		URL:  catInfo.URL,
	}

	// Получаем подкатегории
	subcategories := getPredefinedSubcategories(catInfo.Name)

	if len(subcategories) == 0 {
		// Если нет подкатегорий, парсим напрямую
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
		category.Subcategories = append(category.Subcategories, Subcategory{
			Name: catInfo.Name, URL: categoryURL, Products: products,
		})
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

// parseProductList собирает все продукты через плоский список productlist (режим A)
func parseProductList(sc *ScraperClient, downloader *Downloader, catalog *Catalog, skipImages bool, limit int) {
	fmt.Println("Сбор всех ссылок на продукты с productlist (1-81)...")

	var allLinks []string

	for page := 1; page <= 81; page++ {
		var url string
		if page == 1 {
			url = "https://www.mstpumps.com/products"
		} else {
			url = fmt.Sprintf("https://www.mstpumps.com/productlist-%d", page)
		}

		fmt.Printf("Страница %d/81: %s\n", page, url)

		// Ретрай для страниц с 0 товаров (может быть brotli-сжатие)
		var links []string
		for retry := 0; retry < 3; retry++ {
			if retry > 0 {
				fmt.Printf("  🔄 Повторная попытка %d...\n", retry+1)
				sc.RandomDelay()
				sc.RandomDelay() // двойная пауза перед ретраем
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
				break // получили данные или закончились попытки
			}
			fmt.Printf("  ⚠️ Получено 0 товаров (попытка %d), повторяю...\n", retry+1)
		}

		allLinks = append(allLinks, links...)
		fmt.Printf("  Найдено: %d (всего: %d)\n", len(links), len(allLinks))

		if page < 81 {
			sc.RandomDelay()
		}
	}

	fmt.Printf("\nВсего собрано ссылок: %d\n", len(allLinks))

	// Создаём категорию "All Products" для Mode A
	category := Category{
		Name: "All Products",
		URL:  "https://www.mstpumps.com/products",
	}

	var products []Product
	for i, link := range allLinks {
		if limit > 0 && i >= limit {
			break
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
		fmt.Printf("✅ Продукт %d/%d: %s\n", i+1, len(allLinks), product.Title)

		if i < len(allLinks)-1 {
			sc.RandomDelay()
		}
	}

	category.Subcategories = append(category.Subcategories, Subcategory{
		Name:     "All Products",
		URL:      "https://www.mstpumps.com/products",
		Products: products,
	})
	catalog.Categories = append(catalog.Categories, category)
}

// getPredefinedSubcategories возвращает подкатегории для указанной категории
func getPredefinedSubcategories(categoryName string) []Subcategory {
	// Определяем подкатегории на основе вашего описания
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
		"Submersible Pumps": {
			// Без подкатегорий
		},
		"Borehole Pump": {
			{Name: "Stainless Steel Borehole Pump", URL: "https://www.mstpumps.com/borehole-pump/stainless-steel-borehole-pump/"},
			{Name: "Cast Iron Borehole Pump", URL: "https://www.mstpumps.com/borehole-pump/cast-iron-borehole-pump/"},
		},
	}

	return subcategoryMap[categoryName]
}

// extractCategoryNameFromURL извлекает название категории из URL
func extractCategoryNameFromURL(rawURL string) string {
	parts := strings.Split(strings.TrimRight(rawURL, "/"), "/")
	if len(parts) > 0 {
		name := strings.ReplaceAll(parts[len(parts)-1], "-", " ")
		name = strings.ReplaceAll(name, "_", " ")
		return strings.Title(name)
	}
	return "Unknown"
}
