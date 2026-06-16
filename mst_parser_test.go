package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ==================== ТЕСТЫ ПАРСИНГА ССЫЛОК ====================

func TestParseProductLinksFromListing(t *testing.T) {
	// Тест 1: "Read More" на английском
	html1 := `<html><body>
		<a href="/product1.html">Read More</a>
		<a href="/product2.html">Read More</a>
	</body></html>`
	doc1, _ := goquery.NewDocumentFromReader(strings.NewReader(html1))
	links := ParseProductLinksFromListing(doc1)
	if len(links) != 2 {
		t.Errorf("Ожидалось 2 ссылки, получено %d. Ссылки: %v", len(links), links)
	}

	// Тест 2: Разные варианты "Read More" (пробелы, регистр)
	html2 := `<html><body>
		<a href="/product3.html">Read  More</a>
		<a href="/product4.html">read more</a>
		<a href="/product5.html">Read More</a>
	</body></html>`
	doc2, _ := goquery.NewDocumentFromReader(strings.NewReader(html2))
	links2 := ParseProductLinksFromListing(doc2)
	if len(links2) != 3 {
		t.Errorf("Ожидалось 3 ссылки (с разными пробелами/регистром), получено %d", len(links2))
	}

	// Тест 3: Нет ссылок
	html3 := `<html><body><p>No links here</p></body></html>`
	doc3, _ := goquery.NewDocumentFromReader(strings.NewReader(html3))
	links3 := ParseProductLinksFromListing(doc3)
	if len(links3) != 0 {
		t.Errorf("Ожидалось 0 ссылок, получено %d", len(links3))
	}

	// Тест 4: Фильтрация дубликатов
	html4 := `<html><body>
		<a href="/product1.html">Read More</a>
		<a href="/product1.html">Read More</a>
	</body></html>`
	doc4, _ := goquery.NewDocumentFromReader(strings.NewReader(html4))
	links4 := ParseProductLinksFromListing(doc4)
	if len(links4) != 1 {
		t.Errorf("Ожидался 1 уникальный URL, получено %d", len(links4))
	}

	// Тест 5: Относительные URL -> абсолютные
	html5 := `<html><body>
		<a href="/water-pumps/product.html">Read More</a>
	</body></html>`
	doc5, _ := goquery.NewDocumentFromReader(strings.NewReader(html5))
	links5 := ParseProductLinksFromListing(doc5)
	if len(links5) != 1 || !strings.HasPrefix(links5[0], "https://www.mstpumps.com") {
		t.Errorf("Ожидался абсолютный URL с https://www.mstpumps.com, получен: %s", links5[0])
	}

	// Тест 6: Пропуск навигационных ссылок
	html6 := `<html><body>
		<a href="#section">Read More</a>
		<a href="javascript:void(0)">Read More</a>
		<a href="mailto:test@test.com">Read More</a>
		<a href="tel:+1234567890">Read More</a>
		<a href="/product.html">Read More</a>
	</body></html>`
	doc6, _ := goquery.NewDocumentFromReader(strings.NewReader(html6))
	links6 := ParseProductLinksFromListing(doc6)
	if len(links6) != 1 {
		t.Errorf("Ожидалась 1 валидная ссылка (остальные навигационные), получено %d", len(links6))
	}
}

func TestImageFiltering(t *testing.T) {
	url := "https://www.mstpumps.com/slurry-pumps/energy-efficient-slurry-pump.html"
	sc := NewScraperClient(1500*time.Millisecond, 4000*time.Millisecond, 5)
	product, err := ParseProductPage(sc, url)
	if err != nil {
		t.Fatalf("Ошибка парсинга: %v", err)
	}

	t.Logf("Продукт: %s", product.Title)
	t.Logf("Изображений после фильтрации: %d", len(product.Images))

	// Проверяем, что нет мусорных изображений
	for _, img := range product.Images {
		lower := strings.ToLower(img.SmallRemoteURL)
		for _, pattern := range []string{"share_", "lang", "flag", "erweima", "qrcode", "logo", "banner", "rollpro", "facebook", "twitter"} {
			if strings.Contains(lower, pattern) {
				t.Errorf("Мусорное изображение не отфильтровано: %s (содержит '%s')", img.SmallRemoteURL, pattern)
			}
		}
	}

	// Основное фото должно быть
	found := false
	for _, img := range product.Images {
		if strings.Contains(img.SmallRemoteURL, "energy-efficient-slurry-pump") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Основное фото продукта не найдено!")
	}

	t.Logf("✅ Фильтрация прошла успешно — %d изображений", len(product.Images))
}

func TestParsePagination(t *testing.T) {
	// Тест: пагинация вида "1/81"
	html := `<html><body>
		<div class="pagination">1/81</div>
	</body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	pages, err := ParsePagination(doc)
	if err != nil {
		t.Errorf("Ошибка парсинга пагинации: %v", err)
	}
	if pages != 81 {
		t.Errorf("Ожидалось 81 страница, получено %d", pages)
	}

	// Тест: пустая пагинация
	html2 := `<html><body><p>no pagination</p></body></html>`
	doc2, _ := goquery.NewDocumentFromReader(strings.NewReader(html2))
	pages2, _ := ParsePagination(doc2)
	if pages2 != 0 {
		t.Errorf("Ожидалось 0 страниц (нет пагинации), получено %d", pages2)
	}
}

func TestIsProductURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"https://www.mstpumps.com/water-pumps/product.html", true},
		{"https://www.mstpumps.com/water-pumps/page-2/", false},
		{"https://www.mstpumps.com/", false},
		{"https://www.mstpumps.com/slurry-pumps/", false},
		{"https://www.mstpumps.com/slurry-pumps/heavy-duty-pump/", true},
	}
	for _, tt := range tests {
		result := IsProductURL(tt.url)
		if result != tt.expected {
			t.Errorf("IsProductURL(%s) = %v, ожидалось %v", tt.url, result, tt.expected)
		}
	}
}

func TestGetProductIDFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		// Функция обрезает .html через strings.TrimSuffix
		{"https://www.mstpumps.com/water-pumps/high-head-pump.html", "high-head-pump"},
		{"https://www.mstpumps.com/slurry-pumps/heavy-duty-pump/", "heavy-duty-pump"},
		{"https://www.mstpumps.com/", ""},
	}
	for _, tt := range tests {
		result := GetProductIDFromURL(tt.url)
		if result != tt.expected {
			t.Errorf("GetProductIDFromURL(%s) = %s, ожидалось %s", tt.url, result, tt.expected)
		}
	}
}

// ==================== ТЕСТЫ ПАРСИНГА ПРОДУКТОВ ====================

func TestResolveURL(t *testing.T) {
	base := "https://www.mstpumps.com/water-pumps/product.html"
	tests := []struct {
		relative string
		expected string
	}{
		{"/images/pump.jpg", "https://www.mstpumps.com/images/pump.jpg"},
		{"https://cdn.example.com/img.jpg", "https://cdn.example.com/img.jpg"},
		{"images/pump.jpg", "https://www.mstpumps.com/water-pumps/images/pump.jpg"},
	}
	for _, tt := range tests {
		result := resolveURL(base, tt.relative)
		if result != tt.expected {
			t.Errorf("resolveURL(%s, %s) = %s, ожидалось %s", base, tt.relative, result, tt.expected)
		}
	}
}

func TestIsIconOrLogo(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"https://site.com/logo.png", true},
		{"https://site.com/favicon.ico", true},
		{"https://site.com/facebook-icon.png", true},
		{"https://site.com/product-photo.jpg", false},
		{"https://site.com/pump-image.jpg", false},
		{"https://site.com/arrow-right.png", true},
	}
	for _, tt := range tests {
		result := isIconOrLogo(tt.url)
		if result != tt.expected {
			t.Errorf("isIconOrLogo(%s) = %v, ожидалось %v", tt.url, result, tt.expected)
		}
	}
}

func TestExtractCategoryFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://www.mstpumps.com/water-pumps/multistage-water-pump/high-head-pump.html", "Multistage Water Pump"},
		{"https://www.mstpumps.com/slurry-pumps/heavy-duty-slurry-pump/pump.html", "Heavy Duty Slurry Pump"},
		{"https://www.mstpumps.com/", ""},
		{"https://www.mstpumps.com/single-category/product.html", "Single Category"},
	}
	for _, tt := range tests {
		result := extractCategoryFromURL(tt.url)
		if result != tt.expected {
			t.Errorf("extractCategoryFromURL(%s) = %s, ожидалось %s", tt.url, result, tt.expected)
		}
	}
}

// ==================== ТЕСТЫ STORAGE (JSON + MODX) ====================

func TestConvertToMODXProduct(t *testing.T) {
	product := &Product{
		ProductID:      "test-product",
		Title:          "Test Pump",
		Pagetitle:      "Test Pump",
		Alias:          "test-pump",
		URL:            "https://www.mstpumps.com/test-pump.html",
		Description:    "<p>Test description</p>",
		Specifications: map[string]string{"Flow": "100 m³/h", "Head": "50 m"},
		Images: []ImageSet{
			{SmallRemoteURL: "https://site.com/img.jpg", LargeRemoteURL: "https://site.com/img-large.jpg"},
		},
		SourceURL: "https://www.mstpumps.com/test-pump.html",
		Template:  5,
		Published: true,
	}

	modxProduct := ConvertToMODXProduct(product)

	if modxProduct.Pagetitle != "Test Pump" {
		t.Errorf("Pagetitle = %s, ожидалось 'Test Pump'", modxProduct.Pagetitle)
	}
	if modxProduct.Alias != "test-pump" {
		t.Errorf("Alias = %s, ожидалось 'test-pump'", modxProduct.Alias)
	}
	if modxProduct.Template != 5 {
		t.Errorf("Template = %d, ожидалось 5", modxProduct.Template)
	}
	if !modxProduct.Published {
		t.Errorf("Published = false, ожидалось true")
	}
	if modxProduct.Specifications["Flow"] != "100 m³/h" {
		t.Errorf("Specifications['Flow'] = %s, ожидалось '100 m³/h'", modxProduct.Specifications["Flow"])
	}
	if modxProduct.SourceURL != "https://www.mstpumps.com/test-pump.html" {
		t.Errorf("SourceURL = %s, ожидался оригинальный URL", modxProduct.SourceURL)
	}
}

func TestConvertCatalogToMODXProducts(t *testing.T) {
	catalog := &Catalog{
		Categories: []Category{
			{
				Name: "Water Pumps",
				URL:  "https://www.mstpumps.com/water-pumps/",
				Subcategories: []Subcategory{
					{
						Name: "Multistage Water Pump",
						URL:  "https://www.mstpumps.com/water-pumps/multistage-water-pump/",
						Products: []Product{
							{Title: "Pump 1", Pagetitle: "Pump 1", Alias: "pump-1", URL: "https://...", Description: "desc", Template: 5, Published: true},
							{Title: "Pump 2", Pagetitle: "Pump 2", Alias: "pump-2", URL: "https://...", Description: "desc", Template: 5, Published: true},
						},
					},
				},
			},
		},
	}

	modxProducts := ConvertCatalogToMODXProducts(catalog)
	if len(modxProducts) != 2 {
		t.Errorf("Ожидалось 2 MODX-продукта, получено %d", len(modxProducts))
	}
	if modxProducts[0].Parent != 1 {
		t.Errorf("Parent первого продукта = %d, ожидалось 1", modxProducts[0].Parent)
	}
	if modxProducts[0].ProductCategory != "Multistage Water Pump" {
		t.Errorf("ProductCategory = %s, ожидалось 'Multistage Water Pump'", modxProducts[0].ProductCategory)
	}
	if modxProducts[1].Parent != 1 {
		t.Errorf("Parent второго продукта = %d, ожидалось 1", modxProducts[1].Parent)
	}
}

func TestWrapInHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"<div>test</div>", "<p><div>test</div></p>"},
		{"simple text", "<p>simple text</p>"},
		{"<p>already wrapped</p>", "<p><p>already wrapped</p></p>"},
	}
	for _, tt := range tests {
		result := wrapInHTML(tt.input)
		if result != tt.expected {
			t.Errorf("wrapInHTML(%q) = %q, ожидалось %q", tt.input, result, tt.expected)
		}
	}
}

func TestJSONSerialization(t *testing.T) {
	// Проверка сериализации Product в JSON (roundtrip)
	product := Product{
		Title:       "Test Product",
		URL:         "https://www.mstpumps.com/test.html",
		Description: "<p>Test</p>",
		Specifications: map[string]string{
			"Power": "10 kW",
			"Speed": "1450 rpm",
		},
		Template:  5,
		Published: true,
	}

	data, err := json.MarshalIndent(product, "", "  ")
	if err != nil {
		t.Fatalf("Ошибка маршализации: %v", err)
	}

	// Проверяем, что JSON содержит ожидаемые поля
	jsonStr := string(data)
	if !strings.Contains(jsonStr, "Test Product") {
		t.Error("JSON не содержит название продукта")
	}
	if !strings.Contains(jsonStr, "10 kW") {
		t.Error("JSON не содержит характеристики")
	}
	if !strings.Contains(jsonStr, "specifications") {
		t.Error("JSON не содержит поле specifications")
	}
	if !strings.Contains(jsonStr, "template") {
		t.Error("JSON не содержит template")
	}

	// Обратная десериализация
	var restored Product
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Ошибка десериализации: %v", err)
	}
	if restored.Title != product.Title {
		t.Errorf("Title после roundtrip = %s, ожидалось %s", restored.Title, product.Title)
	}
	if restored.Specifications["Power"] != "10 kW" {
		t.Errorf("Specifications после roundtrip некорректны: Power = %s", restored.Specifications["Power"])
	}

	// Проверяем MODX JSON
	modxProd := ConvertToMODXProduct(&restored)
	modxData, _ := json.MarshalIndent(modxProd, "", "  ")
	modxStr := string(modxData)
	if !strings.Contains(modxStr, "pagetitle") {
		t.Error("MODX JSON не содержит pagetitle")
	}
	if !strings.Contains(modxStr, "content") {
		t.Error("MODX JSON не содержит content")
	}
}

func TestSaveCatalog(t *testing.T) {
	// Используем временный файл
	tmpFile := "_test_catalog.json"
	defer os.Remove(tmpFile) // очистка после теста

	catalog := &Catalog{
		Categories: []Category{
			{
				Name: "Test Category",
				URL:  "https://www.mstpumps.com/test/",
				Subcategories: []Subcategory{
					{
						Name: "Test Sub",
						URL:  "https://www.mstpumps.com/test/sub/",
						Products: []Product{
							{Title: "Test Product", Pagetitle: "Test Product", Alias: "test", URL: "https://...", Template: 5, Published: true},
						},
					},
				},
			},
		},
	}

	// Сохраняем
	err := SaveCatalog(catalog, tmpFile)
	if err != nil {
		t.Fatalf("Ошибка SaveCatalog: %v", err)
	}

	// Читаем и проверяем
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Ошибка чтения файла: %v", err)
	}
	if !strings.Contains(string(data), "Test Product") {
		t.Error("Файл не содержит ожидаемых данных")
	}
	if !strings.Contains(string(data), "Test Category") {
		t.Error("Файл не содержит название категории")
	}

	// Проверяем, что это валидный JSON
	var restored Catalog
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Сохранённый файл не является валидным JSON: %v", err)
	}
	if len(restored.Categories) != 1 {
		t.Errorf("Ожидалась 1 категория, получено %d", len(restored.Categories))
	}
}

// ==================== ТЕСТЫ STRUCT ====================

func TestProductDefaults(t *testing.T) {
	// Проверка значений по умолчанию
	p := Product{
		Title: "Test",
		URL:   "https://www.mstpumps.com/test.html",
	}
	if p.Published != false {
		t.Error("Published должен быть false по умолчанию")
	}
	if p.Specifications != nil {
		t.Error("Specifications должен быть nil по умолчанию")
	}
	if p.Images != nil {
		t.Error("Images должен быть nil по умолчанию")
	}
}

func TestEmptyCatalog(t *testing.T) {
	// Пустой каталог
	catalog := &Catalog{}
	modxProducts := ConvertCatalogToMODXProducts(catalog)
	if len(modxProducts) != 0 {
		t.Errorf("Ожидалось 0 MODX-продуктов для пустого каталога, получено %d", len(modxProducts))
	}
}

func TestCategoryWithMultipleSubcategories(t *testing.T) {
	// Категория с несколькими подкатегориями
	catalog := &Catalog{
		Categories: []Category{
			{
				Name: "Test",
				URL:  "https://test.com/",
				Subcategories: []Subcategory{
					{Name: "Sub1", URL: "https://test.com/sub1/", Products: []Product{{Title: "P1", Pagetitle: "P1", Alias: "p1", URL: "u1", Template: 5, Published: true}}},
					{Name: "Sub2", URL: "https://test.com/sub2/", Products: []Product{{Title: "P2", Pagetitle: "P2", Alias: "p2", URL: "u2", Template: 5, Published: true}}},
				},
			},
		},
	}

	modxProducts := ConvertCatalogToMODXProducts(catalog)
	if len(modxProducts) != 2 {
		t.Errorf("Ожидалось 2 продукта, получено %d", len(modxProducts))
	}
	if modxProducts[0].Parent != 1 {
		t.Errorf("Parent первого = %d, ожидалось 1", modxProducts[0].Parent)
	}
	if modxProducts[0].ProductCategory != "Sub1" {
		t.Errorf("ProductCategory первого = %s, ожидалось 'Sub1'", modxProducts[0].ProductCategory)
	}
	if modxProducts[1].ProductCategory != "Sub2" {
		t.Errorf("ProductCategory второго = %s, ожидалось 'Sub2'", modxProducts[1].ProductCategory)
	}
}
