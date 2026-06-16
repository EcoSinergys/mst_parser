package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SaveCatalog сохраняет структурированный каталог в JSON
func SaveCatalog(catalog *Catalog, filePath string) error {
	// Создаём директорию, если её нет
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("не удалось создать директорию %s: %v", dir, err)
	}

	// Маршализируем с отступами для читаемости
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return fmt.Errorf("ошибка маршализации JSON: %v", err)
	}

	// Записываем в файл
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("ошибка записи файла %s: %v", filePath, err)
	}

	fmt.Printf("✅ Каталог сохранён: %s (%d байт)\n", filePath, len(data))
	return nil
}

// SaveMODXImport сохраняет данные для импорта в MODX 3
func SaveMODXImport(products []MODXProduct, filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("не удалось создать директорию %s: %v", dir, err)
	}

	data, err := json.MarshalIndent(products, "", "  ")
	if err != nil {
		return fmt.Errorf("ошибка маршализации JSON: %v", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("ошибка записи файла %s: %v", filePath, err)
	}

	fmt.Printf("✅ MODX-импорт сохранён: %s (%d байт, %d товаров)\n", filePath, len(data), len(products))
	return nil
}

// ConvertToMODXProduct преобразует Product в MODXProduct
func ConvertToMODXProduct(product *Product) MODXProduct {
	return MODXProduct{
		Pagetitle:       product.Pagetitle,
		Alias:           product.Alias,
		Content:         wrapInHTML(product.Description),
		Parent:          product.Parent,
		Template:        product.Template,
		Published:       product.Published,
		ProductImage:    getLocalProductImage(product),
		ProductCategory: product.ProductCategory,
		SourceURL:       product.SourceURL,
		Specifications:  product.Specifications,
		LocalSmallPath:  getFirstLocalSmall(product),
		LocalLargePath:  getFirstLocalLarge(product),
	}
}

// wrapInHTML оборачивает контент в HTML-теги для MODX
func wrapInHTML(content string) string {
	if content == "" {
		return ""
	}
	// Если контент уже содержит HTML-теги, возвращаем как есть
	if strings.Contains(content, "<") && strings.Contains(content, ">") {
		return "<p>" + content + "</p>"
	}
	return "<p>" + content + "</p>"
}

// getLocalProductImage возвращает путь к первому изображению продукта для MODX
func getLocalProductImage(product *Product) string {
	for _, img := range product.Images {
		if img.LocalSmallPath != "" {
			return img.LocalSmallPath
		}
		if img.LocalLargePath != "" {
			return img.LocalLargePath
		}
	}
	return ""
}

// getFirstLocalSmall возвращает путь к первому маленькому изображению
func getFirstLocalSmall(product *Product) string {
	for _, img := range product.Images {
		if img.LocalSmallPath != "" {
			return img.LocalSmallPath
		}
	}
	return ""
}

// getFirstLocalLarge возвращает путь к первому большому изображению
func getFirstLocalLarge(product *Product) string {
	for _, img := range product.Images {
		if img.LocalLargePath != "" {
			return img.LocalLargePath
		}
	}
	return ""
}

// ConvertCatalogToMODXProducts преобразует весь каталог в плоский список для MODX
func ConvertCatalogToMODXProducts(catalog *Catalog) []MODXProduct {
	var modxProducts []MODXProduct

	categoryIndex := 0
	for _, cat := range catalog.Categories {
		categoryIndex++
		for _, sub := range cat.Subcategories {
			for _, product := range sub.Products {
				modxProduct := ConvertToMODXProduct(&product)
				modxProduct.Parent = categoryIndex
				modxProduct.ProductCategory = sub.Name
				modxProducts = append(modxProducts, modxProduct)
			}
		}
	}

	return modxProducts
}

// PrintSummary выводит сводку по каталогу
func PrintSummary(catalog *Catalog) {
	totalProducts := 0
	fmt.Println("\n═══════════════════════════════════════")
	fmt.Println("📊 СВОДКА ПО КАТАЛОГУ")
	fmt.Println("═══════════════════════════════════════")

	for _, cat := range catalog.Categories {
		fmt.Printf("\n📁 %s (%s)\n", cat.Name, cat.URL)
		if len(cat.Subcategories) == 0 {
			fmt.Println("   (нет подкатегорий)")
			continue
		}
		for _, sub := range cat.Subcategories {
			count := len(sub.Products)
			fmt.Printf("  📂 %s — %d товаров\n", sub.Name, count)
			totalProducts += count
		}
	}

	fmt.Println("\n═══════════════════════════════════════")
	fmt.Printf("📦 Всего категорий: %d\n", len(catalog.Categories))
	fmt.Printf("📦 Всего товаров: %d\n", totalProducts)
	fmt.Println("═══════════════════════════════════════")
}
