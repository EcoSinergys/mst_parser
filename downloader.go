package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DownloadJob описывает одну задачу скачивания изображения.
type DownloadJob struct {
	SrcURL      string // откуда качать (оригинальный URL)
	DestPath    string // куда сохранять (относительный путь из dest_path)
	Category    string
	Subcategory string
	Product     string
}

// DownloadImages скачивает все изображения из списка jobs с указанным уровнем параллельности.
// Возвращает количество успешно скачанных, пропущенных (уже существует) и ошибочных задач.
func DownloadImages(jobs []DownloadJob, concurrency int) (downloaded, skipped, errors int) {
	if len(jobs) == 0 {
		return 0, 0, 0
	}

	// Базовая директория, в которой будут храниться все изображения.
	baseDir := "downloaded_images"
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Ошибка создания %s: %v\n", baseDir, err)
		return 0, 0, len(jobs)
	}

	// HTTP‑клиент с таймаутом и ограничением количества соединений.
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: concurrency,
		},
	}

	jobCh := make(chan DownloadJob, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Запускаем воркеры.
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobCh {
				fullPath := filepath.Join(baseDir, job.DestPath)

				// Пропускаем, если файл уже существует.
				if _, err := os.Stat(fullPath); err == nil {
					mu.Lock()
					skipped++
					mu.Unlock()
					continue
				}

				// Скачиваем.
				resp, err := client.Get(job.SrcURL)
				if err != nil {
					mu.Lock()
					fmt.Printf("  [%d] ❌ %s — %v\n", workerID, job.SrcURL, err)
					errors++
					mu.Unlock()
					continue
				}
				if resp.StatusCode != http.StatusOK {
					resp.Body.Close()
					mu.Lock()
					fmt.Printf("  [%d] ❌ %s — HTTP %d\n", workerID, job.SrcURL, resp.StatusCode)
					errors++
					mu.Unlock()
					continue
				}

				// Создаём директорию, если её нет.
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					resp.Body.Close()
					mu.Lock()
					fmt.Printf("  [%d] ❌ %s — %v\n", workerID, job.SrcURL, err)
					errors++
					mu.Unlock()
					continue
				}

				// Сохраняем файл.
				outFile, err := os.Create(fullPath)
				if err != nil {
					resp.Body.Close()
					mu.Lock()
					fmt.Printf("  [%d] ❌ %s — %v\n", workerID, job.SrcURL, err)
					errors++
					mu.Unlock()
					continue
				}

				// Записываем данные в файл.
				written, _ := io.Copy(outFile, resp.Body)
				outFile.Close()
				resp.Body.Close()

				mu.Lock()
				downloaded++
				progress := float64(downloaded+skipped+errors) / float64(len(jobs)) * 100
				fmt.Printf("  [%d] ✅ [%d/%d] %.0f%% %s → %s (%d bytes)\n",
					workerID, downloaded+skipped+errors, len(jobs), progress, job.SrcURL, job.DestPath, written)
				mu.Unlock()

				// Небольшая задержка между запросами (чтобы не банили).
				time.Sleep(200 * time.Millisecond)
			}
		}(w)
	}

	// Отправляем задачи.
	for _, job := range jobs {
		jobCh <- job
	}
	close(jobCh)
	wg.Wait()

	return downloaded, skipped, errors
}
