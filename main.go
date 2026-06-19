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
	mode := flag.String("mode", "B", "Режим парсинга: B - по категориям, A - через productlist")
	step := flag.String("step", "all", "Шаг: links, products, specs, enrich, all")
	startFrom := flag.Int("start", 0, "С какого продукта начинать (для step=products)")
	count := flag.Int("count", 36, "Сколько продуктов парсить за раз (для step=products)")
	categoryURL := flag.String("category", "", "URL категории (режим B)")
	limit := flag.Int("limit", 0, "Лимит продуктов (0 = без лимита)")
	debugImages := flag.String("debug-images", "", "Показать все изображения с контекстом для указанного URL")
	downloadFlag := flag.Bool("download", false, "Скачать все изображения из catalog_structured.json")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	// Инициализация логгирования
	if err := InitLogger(); err != nil {
		fmt.Printf("❌ Ошибка инициализации логгирования: %v\n", err)
	}
	defer CloseLogger()

	// Режим debug-images — анализ изображений на странице
	if *debugImages != "" {
		analyzePageImages(*debugImages)
		return
	}

	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║    MST Pumps Catalog Parser v2.0            ║")
	fmt.Println("║    Парсер каталога насосов MST Pumps        ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Printf("Режим: %s, Шаг: %s\n", *mode, *step)
	if *limit > 0 {
		fmt.Printf("Лимит продуктов: %d\n", *limit)
	}
	if *step == "products" {
		fmt.Printf("Парсинг продуктов: с %d, количество: %d\n", *startFrom, *count)
	}
	// Режим скачивания изображений
	if *downloadFlag {
		downloadAllImages()
		return
	}

	fmt.Println("📁 Скачивание изображений: отключено (используйте --download)")

	sc := NewScraperClient(1500*time.Millisecond, 4000*time.Millisecond, 5)

	switch *mode {
	case "A":
		processModeA(sc, *step, *startFrom, *count, *limit)
	case "B":
		if *categoryURL != "" {
			parseSingleCategory(sc, &Catalog{}, *categoryURL, *limit)
		} else {
			parseAllCategories(sc, &Catalog{}, *limit)
		}
	case "direct":
		parseDirectProducts(sc)
	default:
		log.Fatalf("❌ Неизвестный режим: %s. Используйте A, B или direct", *mode)
	}
}

// downloadAllImages скачивает все изображения из catalog_structured.json
func downloadAllImages() {
	fmt.Println("\n�� Загрузка всех изображений из catalog_structured.json...")

	data, err := os.ReadFile("catalog_structured.json")
	if err != nil {
		log.Fatalf("❌ Ошибка чтения catalog_structured.json: %v", err)
	}

	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		log.Fatalf("❌ Ошибка парсинга JSON: %v", err)
	}

	var jobs []DownloadJob
	for _, cat := range catalog.Categories {
		for _, sub := range cat.Subcategories {
			for _, prod := range sub.Products {
				for _, img := range prod.Images {
					srcURL := img.SmallRemoteURL
					destPath := img.DestPath
					if srcURL == "" || destPath == "" {
						continue
					}
					// Нормализуем путь — убираем assetsBase, так как DownloadImages сам добавит downloaded_images/
					destPath = strings.TrimPrefix(destPath, "assets/images/")
					jobs = append(jobs, DownloadJob{
						SrcURL:      srcURL,
						DestPath:    destPath,
						Category:    cat.Name,
						Subcategory: sub.Name,
						Product:     prod.Title,
					})
				}
			}
		}
	}

	fmt.Printf("📊 Всего задач на скачивание: %d\n", len(jobs))
	if len(jobs) == 0 {
		fmt.Println("❌ Нет изображений для скачивания.")
		return
	}

	fmt.Println("🚀 Запуск скачивания (5 потоков)...")
	fmt.Println()
	downloaded, skipped, errors := DownloadImages(jobs, 5)

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println("📊 РЕЗУЛЬТАТЫ СКАЧИВАНИЯ")
	fmt.Println("═══════════════════════════════════════════")
	fmt.Printf("✅ Скачано:  %d\n", downloaded)
	fmt.Printf("⏭️  Пропущено (уже есть): %d\n", skipped)
	fmt.Printf("❌ Ошибок:   %d\n", errors)
	fmt.Printf("�� Всего:    %d\n", downloaded+skipped+errors)
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println("📁 Файлы сохранены в папке: downloaded_images/")
}

// analyzePageImages загружает страницу продукта и выводит все изображения с контекстом
func analyzePageImages(url string) {
	fmt.Printf("\n🔍 Анализ изображений на странице:\n%s\n", url)
	fmt.Println(strings.Repeat("=", 80))

	client := NewScraperClient(1500*time.Millisecond, 4000*time.Millisecond, 5)
	resp, err := client.Get(url)
	if err != nil {
		log.Fatalf("❌ Ошибка загрузки: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatalf("❌ Ошибка парсинга: %v", err)
	}

	fmt.Printf("\n%-4s | %-12s | %-80s | %s | %s\n", "№", "БЛОК", "URL", "КОНТЕКСТ", "ALT")
	fmt.Println(strings.Repeat("-", 160))

	idx := 0
	doc.Find("img").Each(func(i int, img *goquery.Selection) {
		src, exists := img.Attr("src")
		if !exists || src == "" {
			return
		}
		idx++

		imgContext := detectImageContext(img, url)

		width, _ := img.Attr("width")
		height, _ := img.Attr("height")
		dims := ""
		if width != "" && height != "" {
			dims = fmt.Sprintf(" [%sx%s]", width, height)
		}

		alt, _ := img.Attr("alt")
		altText := ""
		if alt != "" {
			altText = fmt.Sprintf(" alt=\"%s\"", truncate(alt, 40))
		}

		blockType := identifyBlock(img)
		parentClass := getParentClass(img)

		fmt.Printf("%-4d | %-12s | %-80s | %s%s%s | %s\n",
			idx, blockType, truncate(src, 80), parentClass, dims, altText, truncate(imgContext, 50))
	})

	if idx == 0 {
		fmt.Println("❌ Изображения не найдены")
	}
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("\n📊 Всего изображений: %d\n", idx)
	fmt.Println("\n💡 Легенда блоков:")
	fmt.Println("  [MAIN]    — основная галерея товара (нужно)")
	fmt.Println("  [DESC]    — внутри описания товара (нужно, но не всегда)")
	fmt.Println("  [THUMB]   — миниатюра в галерее (скорее всего дубль)")
	fmt.Println("  [RELATED] — похожие товары (МУСОР)")
	fmt.Println("  [ICON]    — иконка/лого/кнопка (МУСОР)")
	fmt.Println("  [UNKNOWN] — не удалось определить блок")
}

// identifyBlock, detectImageContext and getParentClass are now moved to images.go

// truncate is now moved to images.go

// processModeA — пошаговый режим через productlist
func processModeA(sc *ScraperClient, step string, startFrom, count int, limit int) {
	const linksCacheFile = "_product_links.json"
	const productCacheFile = "_product_cache.json"

	fmt.Println("\n=== Режим верификации A: сбор всех продуктов через productlist ===")

	// ----- ШАГ 1: Сбор ссылок -----
	if step == "all" || step == "links" {
		allLinks := collectAllLinks(sc, linksCacheFile)
		fmt.Printf("\n📊 Итого: %d ссылок сохранено в %s\n", len(allLinks), linksCacheFile)
		if step == "links" {
			fmt.Println("✅ Шаг 'links' завершён. Для парсинга запусти: --step=products")
			return
		}
	}

	// Загружаем ссылки из кэша
	allLinks, err := loadLinksCache(linksCacheFile)
	if err != nil || len(allLinks) == 0 {
		log.Fatalf("❌ Нет кэша ссылок (%s). Сначала запусти --step=links", linksCacheFile)
	}
	fmt.Printf("📦 Загружено %d ссылок из кэша\n", len(allLinks))

	// ----- ШАГ 2: Парсинг продуктов -----
	if step == "all" || step == "products" {
		parseProductsBatch(sc, allLinks, productCacheFile, startFrom, count, limit)
	}

	// ----- ШАГ 3: Дозаполнение характеристик -----
	if step == "specs" {
		fillSpecifications(sc)
	}

	// ----- ШАГ 4: Обогащение путями + порядком -----
	if step == "enrich" {
		enrichExistingCatalog()
	}
}

// collectAllLinks собирает ссылки со всех 81 страниц productlist
func collectAllLinks(sc *ScraperClient, cacheFile string) []string {
	fmt.Println("\n📌 ШАГ 1: Сбор ссылок с productlist (1-81)...")
	fmt.Println("   Каждая страница: ~12 товаров")
	fmt.Println("   Всего ожидается: ~963 товара")

	var allLinks []string
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
		fmt.Printf("  ✅ Найдено: %d (всего: %d)\n", len(links), len(allLinks))
		if page < 81 {
			sc.RandomDelay()
		}
	}
	if err := saveLinksCache(allLinks, cacheFile); err != nil {
		log.Printf("⚠️ Ошибка сохранения кэша: %v", err)
	}
	fmt.Printf("💾 Ссылки сохранены: %s (%d шт.)\n", cacheFile, len(allLinks))
	return allLinks
}

// parseProductsBatch парсит продукты порциями
func parseProductsBatch(sc *ScraperClient, allLinks []string, cacheFile string, startFrom, count int, limit int) {
	fmt.Println("\n📌 ШАГ 2: Парсинг продуктов")
	parsedCache, _ := loadProductCache(cacheFile)
	fmt.Printf("�� Ранее спарсено: %d продуктов\n", len(parsedCache))

	totalNew := len(allLinks)
	if limit > 0 && limit < totalNew {
		totalNew = limit
	}
	end := startFrom + count
	if end > totalNew {
		end = totalNew
	}
	fmt.Printf("📊 Парсинг продуктов %d — %d из %d\n", startFrom+1, end, len(allLinks))
	if startFrom >= len(allLinks) {
		fmt.Println("✅ Все продукты уже спарсены!")
		return
	}

	products := make([]Product, 0)
	var newlyParsed []string
	for i := startFrom; i < end && i < len(allLinks); i++ {
		link := allLinks[i]
		if limit > 0 && i >= limit {
			break
		}
		if parsedCache[link] {
			fmt.Printf("⏭️ [%d/%d] Пропущен (уже есть)\n", i+1, len(allLinks))
			continue
		}
		fmt.Printf("🔍 [%d/%d] %s\n", i+1, len(allLinks), link)
		product, err := ParseProductPage(sc, link)
		if err != nil {
			log.Printf("⚠️ Ошибка: %v", err)
			continue
		}
		products = append(products, *product)
		newlyParsed = append(newlyParsed, link)
		fmt.Printf("✅ [%d/%d] %s\n", i+1, len(allLinks), product.Title)
		if i < end-1 {
			sc.RandomDelay()
		}
	}

	if len(newlyParsed) > 0 {
		allParsed := make([]string, 0, len(parsedCache)+len(newlyParsed))
		for k := range parsedCache {
			allParsed = append(allParsed, k)
		}
		allParsed = append(allParsed, newlyParsed...)
		saveProductCache(allParsed, cacheFile)
		fmt.Printf("💾 Кэш обновлён: %s (%d продуктов)\n", cacheFile, len(allParsed))

		mergeProductsToCatalog(products)
	}

	fmt.Printf("\n📊 Итого спарсено в этом запуске: %d\n", len(newlyParsed))
	fmt.Printf("📊 Осталось: %d\n", len(allLinks)-len(parsedCache)-len(newlyParsed))
	fmt.Printf("💡 Чтобы продолжить, запусти: --step=products --start=%d --count=%d\n", end, count)
}

// enrichExistingCatalog читает существующий catalog_structured.json и обогащает его путями/порядком
func enrichExistingCatalog() {
	fmt.Println("\n📌 ШАГ 4: Обогащение каталога путями и порядком")

	data, err := os.ReadFile("catalog_structured.json")
	if err != nil {
		log.Fatalf("❌ Нет файла catalog_structured.json. Сначала запусти --step=products")
	}
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		log.Fatalf("❌ Ошибка парсинга catalog_structured.json: %v", err)
	}

	EnrichCatalogWithDestPaths(&catalog)

	SaveCatalog(&catalog, "catalog_structured.json")
	modxProducts := ConvertCatalogToMODXProducts(&catalog)
	SaveMODXImport(modxProducts, "modx_import.json")
	PrintSummary(&catalog)

	fmt.Println("✅ Шаг 'enrich' завершён! Пути и порядок проставлены.")
}

// fillSpecifications — ШАГ 3: дозаполняет характеристики у уже спарсенных продуктов
func fillSpecifications(sc *ScraperClient) {
	fmt.Println("\n📌 ШАГ 3: Дозаполнение характеристик")

	data, err := os.ReadFile("catalog_structured.json")
	if err != nil {
		log.Fatalf("❌ Нет файла catalog_structured.json. Сначала запусти --step=products")
	}
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		log.Fatalf("❌ Ошибка парсинга catalog_structured.json: %v", err)
	}

	updated := 0
	skipped := 0
	for ci := range catalog.Categories {
		for si := range catalog.Categories[ci].Subcategories {
			for pi := range catalog.Categories[ci].Subcategories[si].Products {
				prod := &catalog.Categories[ci].Subcategories[si].Products[pi]
				if len(prod.Specifications) > 0 {
					skipped++
					continue
				}
				fmt.Printf("🔍 [%d] %s\n", pi+1, prod.URL)
				resp, err := sc.Get(prod.URL)
				if err != nil {
					log.Printf("⚠️ Ошибка загрузки: %v", err)
					continue
				}
				doc, err := goquery.NewDocumentFromReader(resp.Body)
				resp.Body.Close()
				if err != nil {
					log.Printf("⚠️ Ошибка парсинга HTML: %v", err)
					continue
				}
				specs := make(map[string]string)
				doc.Find("p, div, span, td, li").Each(func(i int, s *goquery.Selection) {
					s.Find("strong, b").Each(func(j int, strong *goquery.Selection) {
						text := strings.TrimSpace(strong.Text())
						if !strings.HasSuffix(text, ":") && !strings.Contains(text, ":") {
							return
						}
						parentHtml, err := strong.Parent().Html()
						if err != nil {
							return
						}
						cleanKey := strings.TrimRight(strings.TrimSpace(text), ":")
						strongHtml, err := goquery.OuterHtml(strong)
						if err != nil {
							return
						}
						parts := strings.SplitN(parentHtml, strongHtml, 2)
						if len(parts) == 2 {
							value := strings.TrimSpace(stripHTML(parts[1]))
							value = strings.TrimRight(value, ".,; ")
							if cleanKey != "" && value != "" && len(value) < 200 {
								specs[cleanKey] = value
							}
						}
					})
				})
				prod.Specifications = specs
				fmt.Printf("  ✅ Найдено характеристик: %d\n", len(specs))
				updated++
				sc.RandomDelay()
			}
		}
	}

	if updated > 0 {
		EnrichCatalogWithDestPaths(&catalog)
		SaveCatalog(&catalog, "catalog_structured.json")
		modxProducts := ConvertCatalogToMODXProducts(&catalog)
		SaveMODXImport(modxProducts, "modx_import.json")
	}
	fmt.Printf("\n📊 Обновлено: %d, пропущено (уже были): %d\n", updated, skipped)
	fmt.Println("✅ Шаг 'specs' завершён!")
}

// mergeProductsToCatalog добавляет новые продукты в существующий каталог
func mergeProductsToCatalog(products []Product) {
	if len(products) == 0 {
		return
	}
	existing := &Catalog{}
	if data, err := os.ReadFile("catalog_structured.json"); err == nil {
		json.Unmarshal(data, existing)
	}
	if len(existing.Categories) > 0 && len(existing.Categories[0].Subcategories) > 0 {
		existing.Categories[0].Subcategories[0].Products = append(existing.Categories[0].Subcategories[0].Products, products...)
	} else {
		existing = &Catalog{Categories: []Category{{Name: "All Products", URL: "https://www.mstpumps.com/products",
			Subcategories: []Subcategory{{Name: "All Products", URL: "https://www.mstpumps.com/products",
				Products: products}}}}}
	}
	EnrichCatalogWithDestPaths(existing)
	SaveCatalog(existing, "catalog_structured.json")
	SaveMODXImport(ConvertCatalogToMODXProducts(existing), "modx_import.json")
	PrintSummary(existing)
}

// -------------------- Кэш-функции --------------------

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

func saveLinksCache(links []string, filePath string) error {
	data, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

func loadProductCache(filePath string) (map[string]bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return make(map[string]bool), nil
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

func saveProductCache(urls []string, filePath string) error {
	data, err := json.MarshalIndent(urls, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// -------------------- Режим B (категории) --------------------

func parseAllCategories(sc *ScraperClient, catalog *Catalog, limit int) {
	fmt.Println("\n=== Режим B: Парсинг по дереву категорий ===")
	globalProductCount := 0
	for _, catInfo := range categoryStructure {
		fmt.Printf("\n📁 Категория: %s (%s)\n", catInfo.Name, catInfo.URL)
		category := Category{Name: catInfo.Name, URL: catInfo.URL}
		if catInfo.HasChildren {
			sc.Get(catInfo.URL)
			subcategories := getPredefinedSubcategories(catInfo.Name)
			if len(subcategories) == 0 {
				fmt.Printf("  ⚠️ Нет подкатегорий для %s\n", catInfo.Name)
				continue
			}
			for _, sub := range subcategories {
				fmt.Printf("\n  📂 Подкатегория: %s (%s)\n", sub.Name, sub.URL)
				links, err := ScrapeSubcategoryLinks(sc, sub.URL)
				if err != nil {
					log.Printf("  ⚠️ Ошибка: %v", err)
					continue
				}
				fmt.Printf("  Найдено продуктов: %d\n", len(links))
				var products []Product
				for i, link := range links {
					if limit > 0 && globalProductCount >= limit {
						break
					}
					product, err := ParseProductPage(sc, link)
					if err != nil {
						continue
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
			links, err := ScrapeCategoryLinks(sc, catInfo.URL)
			if err != nil {
				continue
			}
			var products []Product
			for i, link := range links {
				if limit > 0 && globalProductCount >= limit {
					break
				}
				product, err := ParseProductPage(sc, link)
				if err != nil {
					continue
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
	EnrichCatalogWithDestPaths(catalog)
	SaveCatalog(catalog, "catalog_structured.json")
	SaveMODXImport(ConvertCatalogToMODXProducts(catalog), "modx_import.json")
	PrintSummary(catalog)
}

func parseSingleCategory(sc *ScraperClient, catalog *Catalog, categoryURL string, limit int) {
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
				products = append(products, *product)
				sc.RandomDelay()
			}
			sub.Products = products
			category.Subcategories = append(category.Subcategories, sub)
		}
	}
	catalog.Categories = append(catalog.Categories, category)
	EnrichCatalogWithDestPaths(catalog)
	SaveCatalog(catalog, "catalog_structured.json")
	SaveMODXImport(ConvertCatalogToMODXProducts(catalog), "modx_import.json")
	PrintSummary(catalog)
}

func getPredefinedSubcategories(categoryName string) []Subcategory {
	m := map[string][]Subcategory{
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
	return m[categoryName]
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
