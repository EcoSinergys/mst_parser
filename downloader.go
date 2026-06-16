package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Downloader управляет скачиванием изображений
type Downloader struct {
	baseDir  string
	smallDir string
	largeDir string
	client   *http.Client
}

// NewDownloader создаёт новый загрузчик изображений
func NewDownloader(baseDir string) *Downloader {
	smallDir := filepath.Join(baseDir, "small")
	largeDir := filepath.Join(baseDir, "large")

	return &Downloader{
		baseDir:  baseDir,
		smallDir: smallDir,
		largeDir: largeDir,
		client: &http.Client{
			Timeout: time.Duration(30) * time.Second,
		},
	}
}

// InitDirs создаёт все необходимые директории
func (d *Downloader) InitDirs() error {
	dirs := []string{d.baseDir, d.smallDir, d.largeDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("не удалось создать директорию %s: %v", dir, err)
		}
	}
	fmt.Printf("📁 Директории для изображений созданы: %s\n", d.baseDir)
	return nil
}

// DownloadImage скачивает изображение по URL и сохраняет в указанную директорию
// Возвращает относительный путь к сохранённому файлу
func (d *Downloader) DownloadImage(imageURL string, productID string, suffix string) (string, error) {
	if imageURL == "" {
		return "", fmt.Errorf("пустой URL")
	}

	// Определяем расширение файла
	ext := getExtension(imageURL)
	if ext == "" {
		ext = ".jpg"
	}

	// Формируем имя файла
	filename := fmt.Sprintf("%s_%s%s", sanitizeFilename(productID), suffix, ext)

	// Определяем директорию
	targetDir := d.largeDir
	if suffix == "small" {
		targetDir = d.smallDir
	}

	filePath := filepath.Join(targetDir, filename)
	relativePath := filepath.Join(d.baseDir, suffix, filename)

	// Проверяем, существует ли уже файл
	if _, err := os.Stat(filePath); err == nil {
		fmt.Printf("  📸 Файл уже существует: %s\n", relativePath)
		return relativePath, nil
	}

	// Скачиваем
	fmt.Printf("  📥 Скачивание: %s\n", imageURL)

	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса: %v", err)
	}

	// Добавляем базовые заголовки
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.mstpumps.com/")

	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка скачивания: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("сервер вернул код %d", resp.StatusCode)
	}

	// Создаём файл
	outFile, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("ошибка создания файла %s: %v", filePath, err)
	}
	defer outFile.Close()

	// Записываем данные
	written, err := io.Copy(outFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка записи файла: %v", err)
	}

	fmt.Printf("  ✅ Сохранено: %s (%d байт)\n", relativePath, written)
	return relativePath, nil
}

// DownloadProductImages скачивает все изображения продукта
func (d *Downloader) DownloadProductImages(product *Product) error {
	var updatedImages []ImageSet

	for _, img := range product.Images {
		// Скачиваем маленькое изображение
		localSmall, err := d.DownloadImage(img.SmallRemoteURL, product.ProductID, "small")
		if err != nil {
			fmt.Printf("  ⚠️ Не удалось скачать small: %v\n", err)
			localSmall = ""
		}

		// Скачиваем большое изображение
		localLarge, err := d.DownloadImage(img.LargeRemoteURL, product.ProductID, "large")
		if err != nil {
			fmt.Printf("  ⚠️ Не удалось скачать large: %v\n", err)
			localLarge = ""
		}

		updatedImages = append(updatedImages, ImageSet{
			SmallRemoteURL: img.SmallRemoteURL,
			LargeRemoteURL: img.LargeRemoteURL,
			LocalSmallPath: localSmall,
			LocalLargePath: localLarge,
		})
	}

	product.Images = updatedImages
	return nil
}

// getExtension возвращает расширение файла из URL
func getExtension(imageURL string) string {
	// Извлекаем путь из URL (убираем параметры)
	if idx := strings.Index(imageURL, "?"); idx != -1 {
		imageURL = imageURL[:idx]
	}

	ext := filepath.Ext(imageURL)
	ext = strings.ToLower(ext)

	// Проверяем, что это изображение
	validExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg"}
	for _, ve := range validExts {
		if ext == ve {
			return ext
		}
	}

	return ".jpg"
}

// sanitizeFilename очищает строку для использования в имени файла
func sanitizeFilename(name string) string {
	// Заменяем недопустимые символы
	replacer := strings.NewReplacer(
		"/", "-", "\\", "-", " ", "-",
		":", "-", "*", "-", "?", "-",
		"\"", "-", "<", "-", ">", "-",
		"|", "-", "(", "-", ")", "-",
	)
	return replacer.Replace(name)
}
