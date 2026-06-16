//go:build ignore
// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║    MST Parser — Image Uploader              ║")
	fmt.Println("║    Загрузка изображений по JSON-матрице     ║")
	fmt.Println("╚══════════════════════════════════════════════╝")

	if len(os.Args) < 2 {
		fmt.Println("Использование: go run uploader.go catalog_structured.json [параллельность]")
		fmt.Println("  catalog_structured.json — путь к JSON-матрице")
		fmt.Println("  параллельность — количество одновременных загрузок (по умолч. 5)")
		os.Exit(1)
	}

	jsonPath := os.Args[1]
	concurrency := 5
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &concurrency)
	}

	// Читаем JSON
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

	// Собираем все задачи на скачивание
	type DownloadJob struct {
		SrcURL      string // откуда качать (оригинальный URL)
		DestPath    string // куда сохранять (относительный путь из dest_path)
		Category    string
		Subcategory string
		Product     string
	}

	jobs := make([]DownloadJob, 0)
	totalImages := 0

	for _, cat := range catalog.Categories {
		for _, sub := range cat.Subcategories {
			for _, prod := range sub.Products {
				for _, img := range prod.Images {
					totalImages++
					// Берём large URL если есть, иначе small
					srcURL := img.LargeRemoteURL
					if srcURL == "" {
						srcURL = img.SmallRemoteURL
					}
					// Определяем путь сохранения
					destPath := img.DestPath
					if destPath == "" {
						// Если нет dest_path — генерируем из slug'ов
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

	// Создаём базовую директорию
	baseDir := "downloaded_images"
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Ошибка создания %s: %v\n", baseDir, err)
		os.Exit(1)
	}

	// Запускаем worker'ов
	start := time.Now()
	jobCh := make(chan DownloadJob, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	downloaded := 0
	errors := 0
	skipped := 0

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: concurrency,
		},
	}

	// Worker'ы
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobCh {
				// Полный путь к файлу
				fullPath := filepath.Join(baseDir, job.DestPath)

				// Пропускаем, если уже есть
				if _, err := os.Stat(fullPath); err == nil {
					mu.Lock()
					skipped++
					mu.Unlock()
					continue
				}

				// Скачиваем
				resp, err := client.Get(job.SrcURL)
				if err != nil {
					mu.Lock()
					fmt.Printf("  [%d] ❌ %s — %v\n", workerID, job.SrcURL, err)
					errors++
					mu.Unlock()
					continue
				}

				if resp.StatusCode != 200 {
					resp.Body.Close()
					mu.Lock()
					fmt.Printf("  [%d] ❌ %s — HTTP %d\n", workerID, job.SrcURL, resp.StatusCode)
					errors++
					mu.Unlock()
					continue
				}

				// Создаём директорию
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					resp.Body.Close()
					mu.Lock()
					fmt.Printf("  [%d] ❌ %s — %v\n", workerID, job.SrcURL, err)
					errors++
					mu.Unlock()
					continue
				}

				// Сохраняем файл
				outFile, err := os.Create(fullPath)
				if err != nil {
					resp.Body.Close()
					mu.Lock()
					fmt.Printf("  [%d] ❌ %s — %v\n", workerID, job.SrcURL, err)
					errors++
					mu.Unlock()
					continue
				}

				written, _ := io.Copy(outFile, resp.Body)
				outFile.Close()
				resp.Body.Close()

				mu.Lock()
				downloaded++
				progress := float64(downloaded+skipped+errors) / float64(len(jobs)) * 100
				fmt.Printf("  [%d] ✅ [%d/%d] %.0f%% %s → %s (%d bytes)\n",
					workerID, downloaded+skipped+errors, len(jobs), progress,
					job.SrcURL, job.DestPath, written)
				mu.Unlock()

				// Небольшая задержка между запросами (чтобы не банили)
				time.Sleep(200 * time.Millisecond)
			}
		}(w)
	}

	// Отправляем задачи
	for _, job := range jobs {
		jobCh <- job
	}
	close(jobCh)
	wg.Wait()

	elapsed := time.Since(start)
	fmt.Printf("\n═══════════════════════════════════════\n")
	fmt.Printf("📊 ИТОГ ЗАГРУЗКИ\n")
	fmt.Printf("📦 Всего: %d\n", len(jobs))
	fmt.Printf("✅ Скачано: %d\n", downloaded)
	fmt.Printf("⏭️ Пропущено (уже есть): %d\n", skipped)
	fmt.Printf("❌ Ошибок: %d\n", errors)
	fmt.Printf("⏱ Время: %v\n", elapsed)
	fmt.Printf("📁 Папка: %s\n", baseDir)
	fmt.Println("═══════════════════════════════════════")
}
