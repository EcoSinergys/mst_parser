package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const assetsBase = "assets/images"

// SaveCatalog сохраняет структурированный каталог в JSON
func SaveCatalog(catalog *Catalog, filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("не удалось создать директорию %s: %v", dir, err)
	}

	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return fmt.Errorf("ошибка маршализации JSON: %v", err)
	}

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
	// Генерируем правильные пути для MODX
	var modxImages []MODXImage
	for i, img := range product.Images {
		// Если есть dest_path — используем его, иначе small_remote_url
		src := img.DestPath
		if src == "" {
			src = img.SmallRemoteURL
		}
		modxImages = append(modxImages, MODXImage{
			Src:       src,
			Alt:       product.Pagetitle,
			MenuIndex: i + 1,
		})
	}

	// Первое изображение — как основное
	mainImage := ""
	if len(modxImages) > 0 {
		mainImage = modxImages[0].Src
	}

	return MODXProduct{
		Pagetitle:       product.Pagetitle,
		Alias:           product.Alias,
		Content:         wrapInHTML(product.Description),
		Parent:          product.Parent,
		Template:        product.Template,
		Published:       product.Published,
		MenuIndex:       product.MenuIndex,
		ProductImage:    mainImage,
		ProductCategory: product.ProductCategory,
		SourceURL:       product.SourceURL,
		Specifications:  product.Specifications,
		Images:          modxImages,
	}
}

// ComputeDestPath вычисляет целевой путь для изображения в MODX
// Формат: assets/images/{category_slug}/{subcategory_slug}/{product_slug}/{index:02d}.jpg
func ComputeDestPath(categorySlug, subcategorySlug, productSlug string, index int) string {
	ext := ".jpg" // по умолчанию jpg
	parts := []string{assetsBase}
	if categorySlug != "" {
		parts = append(parts, categorySlug)
	}
	if subcategorySlug != "" {
		parts = append(parts, subcategorySlug)
	}
	if productSlug != "" {
		parts = append(parts, productSlug)
	}
	filename := fmt.Sprintf("%02d%s", index, ext)
	parts = append(parts, filename)
	return strings.Join(parts, "/")
}

// ComputeCategoryImagePath вычисляет путь к изображению категории
func ComputeCategoryImagePath(categorySlug string) string {
	return fmt.Sprintf("%s/%s/category.jpg", assetsBase, categorySlug)
}

// ComputeSubcategoryImagePath вычисляет путь к изображению подкатегории
func ComputeSubcategoryImagePath(categorySlug, subcategorySlug string) string {
	return fmt.Sprintf("%s/%s/%s/subcategory.jpg", assetsBase, categorySlug, subcategorySlug)
}

// EnrichCatalogWithDestPaths заполняет dest_path для всех изображений в каталоге
func EnrichCatalogWithDestPaths(catalog *Catalog) {
	catIdx := 0
	for ci := range catalog.Categories {
		catIdx++
		cat := &catalog.Categories[ci]
		cat.MenuIndex = catIdx
		if cat.Slug == "" {
			cat.Slug = slugify(cat.Name)
		}
		// Изображение категории — из первого товара первой подкатегории
		if cat.Image == "" && len(cat.Subcategories) > 0 && len(cat.Subcategories[0].Products) > 0 {
			if len(cat.Subcategories[0].Products[0].Images) > 0 {
				cat.Image = ComputeCategoryImagePath(cat.Slug)
			}
		}

		subIdx := 0
		for si := range cat.Subcategories {
			subIdx++
			sub := &cat.Subcategories[si]
			sub.MenuIndex = subIdx
			if sub.Slug == "" {
				sub.Slug = slugify(sub.Name)
			}
			// Изображение подкатегории — из первого товара
			if sub.Image == "" && len(sub.Products) > 0 && len(sub.Products[0].Images) > 0 {
				sub.Image = ComputeSubcategoryImagePath(cat.Slug, sub.Slug)
			}

			for pi := range sub.Products {
				prod := &sub.Products[pi]
				prod.MenuIndex = pi + 1
				prodSlug := prod.Alias
				if prodSlug == "" {
					prodSlug = slugify(prod.Title)
				}

				for ii := range prod.Images {
					img := &prod.Images[ii]
					img.MenuIndex = ii + 1
					img.DestPath = ComputeDestPath(cat.Slug, sub.Slug, prodSlug, ii+1)
				}
			}
		}
	}
}

// wrapInHTML оборачивает контент в HTML-теги для MODX
func wrapInHTML(content string) string {
	if content == "" {
		return ""
	}
	if strings.Contains(content, "<") && strings.Contains(content, ">") {
		return "<p>" + content + "</p>"
	}
	return "<p>" + content + "</p>"
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
		fmt.Printf("   Slug: %s, Image: %s, MenuIndex: %d\n", cat.Slug, cat.Image, cat.MenuIndex)
		if len(cat.Subcategories) == 0 {
			fmt.Println("   (нет подкатегорий)")
			continue
		}
		for _, sub := range cat.Subcategories {
			count := len(sub.Products)
			fmt.Printf("  📂 %s — %d товаров (slug: %s, idx: %d)\n", sub.Name, count, sub.Slug, sub.MenuIndex)
			totalProducts += count
		}
	}

	fmt.Println("\n═══════════════════════════════════════")
	fmt.Printf("📦 Всего категорий: %d\n", len(catalog.Categories))
	fmt.Printf("📦 Всего товаров: %d\n", totalProducts)
	fmt.Println("═══════════════════════════════════════")
}
