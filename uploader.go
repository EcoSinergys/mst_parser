//go:build ignore
// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// DownloadJob is the same struct defined in downloader.go.
// Keeping it here for compatibility, but the implementation is now in DownloadImages.
type DownloadJob struct {
	SrcURL      string // откуда качать (оригинальный URL)
	DestPath    string // куда сохранять (относительный путь из dest_path)
	Category    string
	Subcategory string
	Product     string
}

// DownloadImages is imported from downloader.go and used here.
// The implementation is now centralized in downloader.go.

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Использование: go run uploader.go catalog_structured.json [параллельность]")
		fmt.Println("  catalog_structured.json — путь к JSON-матрице")
		fmt.Println("  параллельность — количество одновременных загрузок (по умолчанию 5)")
		os.Exit(1)
	}

	jsonPath := os.Args[1]
	concurrency := 5
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &concurrency)
	}

	// Read JSON file.
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Ошибка чтения %s: %v\n", jsonPath, err)
		os.Exit(1)
	}
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Ошибка парсинга JSON: %v\n", err)
		os.Exit(1)
	}

	// Collect all download jobs.
	jobs := make([]DownloadJob, 0)
	totalImages := 0

	for _, cat := range catalog.Categories {
		for _, sub := range cat.Subcategories {
			for _, prod := range sub.Products {
				for _, img := range prod.Images {
					totalImages++
					srcURL := img.LargeRemoteURL
					if srcURL == "" {
						srcURL = img.SmallRemoteURL
					}
					destPath := img.DestPath
					if destPath == "" {
						destPath = fmt.Sprintf("assets/images/%s/%s/%s/%02d.jpg",
							cat.Slug, sub.Slug, prod.Alias, len(jobs)+1)
					}
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

	fmt.Printf("\n📊 Всего изображений: %d\n", totalImages)
	fmt.Printf("📊 Задач на скачивание: %d\n", len(jobs))
	fmt.Printf("🔀 Параллельность: %d\n\n", concurrency)

	if len(jobs) == 0 {
		fmt.Println("✅ Нет изображений для скачивания.")
		return
	}

	// Run the download process using the centralized DownloadImages function.
	downloaded, skipped, errors := DownloadImages(jobs, concurrency)

	// Summary.
	fmt.Printf("\n═══════════════════════════════════════\n")
	fmt.Printf("📊 ИТОГ ЗАГРУЗКИ\n")
	fmt.Printf("📦 Всего: %d\n", len(jobs))
	fmt.Printf("✅ Скачано: %d\n", downloaded)
	fmt.Printf("⏭️ Пропущено (уже есть): %d\n", skipped)
	fmt.Printf("❌ Ошибок: %d\n", errors)
	fmt.Printf("⏱ Время: %v\n", time.Since(time.Now()).Seconds())
	fmt.Printf("📁 Папка: downloaded_images\n")
	fmt.Println("═══════════════════════════════════════")
}
