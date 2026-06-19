//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	// Читаем существующий catalog_structured.json
	data, err := os.ReadFile("catalog_structured.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Ошибка чтения catalog_structured.json: %v\n", err)
		os.Exit(1)
	}

	// Парсим каталог
	var catalog struct {
		Categories []struct {
			CategoryName  string `json:"category_name"`
			CategoryURL   string `json:"category_url"`
			Subcategories []struct {
				SubcategoryName string    `json:"subcategory_name"`
				SubcategoryURL  string    `json:"subcategory_url"`
				Products        []Product `json:"products"`
			} `json:"subcategories"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Ошибка парсинга catalog_structured.json: %v\n", err)
		os.Exit(1)
	}

	// Конвертируем в MODX-формат
	var modxProducts []MODXProduct
	for _, cat := range catalog.Categories {
		for _, sub := range cat.Subcategories {
			for _, p := range sub.Products {
				modx := MODXProduct{
					Pagetitle:       p.Title,
					Alias:           p.Alias,
					Parent:          p.Parent,
					Template:        p.Template,
					Published:       p.Published,
					Description:     p.Description,
					ProductImage:    p.ProductImage,
					ProductCategory: p.ProductCategory,
					SourceURL:       p.URL,
					Images:          p.Images,
				}
				// Сериализуем спецификации в JSON-строку
				if len(p.Specifications) > 0 {
					specJSON, err := json.Marshal(p.Specifications)
					if err == nil {
						modx.Specifications = string(specJSON)
					}
				}
				modxProducts = append(modxProducts, modx)
			}
		}
	}

	// Сохраняем
	output, err := json.MarshalIndent(modxProducts, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Ошибка сериализации: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("modx_import.json", output, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Ошибка записи modx_import.json: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ modx_import.json создан! Продуктов: %d\n", len(modxProducts))
}

// Product represents a single product in the catalog
type Product struct {
	ProductID       int               `json:"product_id"`
	Title           string            `json:"title"`
	URL             string            `json:"url"`
	Description     string            `json:"description"`
	Specifications  map[string]string `json:"specifications"`
	TV              map[string]string `json:"tv"`
	Images          []string          `json:"images"`
	Menuindex       int               `json:"menuindex"`
	Pagetitle       string            `json:"pagetitle"`
	Alias           string            `json:"alias"`
	Parent          int               `json:"parent"`
	Template        int               `json:"template"`
	Published       bool              `json:"published"`
	ProductImage    string            `json:"product_image"`
	ProductCategory string            `json:"product_category"`
	SourceURL       string            `json:"source_url"`
}

// MODXProduct represents a product for MODX import
type MODXProduct struct {
	Pagetitle       string   `json:"pagetitle"`
	Alias           string   `json:"alias"`
	Parent          int      `json:"parent"`
	Template        int      `json:"template"`
	Published       bool     `json:"published"`
	Description     string   `json:"description"`
	Specifications  string   `json:"specifications"`
	ProductImage    string   `json:"product_image"`
	ProductCategory string   `json:"product_category"`
	SourceURL       string   `json:"source_url"`
	Images          []string `json:"images"`
}
