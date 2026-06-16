package main

// ImageSet содержит ссылки на изображения продукта
type ImageSet struct {
	SmallRemoteURL string `json:"small_remote_url"`
	LargeRemoteURL string `json:"large_remote_url"`
	LocalSmallPath string `json:"local_small_path"`
	LocalLargePath string `json:"local_large_path"`
}

// Product содержит полную информацию о продукте
type Product struct {
	ProductID      string            `json:"product_id"`
	Title          string            `json:"title"`
	URL            string            `json:"url"`
	Description    string            `json:"description"`
	Specifications map[string]string `json:"specifications"`
	Images         []ImageSet        `json:"images"`

	// Поля для импорта в MODX 3
	Pagetitle       string `json:"pagetitle"`
	Alias           string `json:"alias"`
	Parent          int    `json:"parent"`
	Template        int    `json:"template"`
	Published       bool   `json:"published"`
	ProductImage    string `json:"product_image"`
	ProductCategory string `json:"product_category"`
	SourceURL       string `json:"source_url"`
}

// Subcategory содержит подкатегорию и её продукты
type Subcategory struct {
	Name     string    `json:"subcategory_name"`
	URL      string    `json:"subcategory_url"`
	Products []Product `json:"products"`
}

// Category содержит категорию с подкатегориями
type Category struct {
	Name          string        `json:"category_name"`
	URL           string        `json:"category_url"`
	Subcategories []Subcategory `json:"subcategories"`
}

// Catalog содержит весь каталог
type Catalog struct {
	Categories []Category `json:"categories"`
}

// MODXProduct представляет структуру для импорта в MODX 3
type MODXProduct struct {
	Pagetitle       string            `json:"pagetitle"`
	Alias           string            `json:"alias"`
	Content         string            `json:"content"`
	Parent          int               `json:"parent"`
	Template        int               `json:"template"`
	Published       bool              `json:"published"`
	ProductImage    string            `json:"product_image"`
	ProductCategory string            `json:"product_category"`
	SourceURL       string            `json:"source_url"`
	Specifications  map[string]string `json:"specifications,omitempty"`
	LocalSmallPath  string            `json:"local_small_path,omitempty"`
	LocalLargePath  string            `json:"local_large_path,omitempty"`
}

// PageInfo содержит информацию о пагинации
type PageInfo struct {
	CurrentPage int
	TotalPages  int
	ProductURLs []string
}

// CategoryInfo содержит информацию о категории для конфигурации
type CategoryInfo struct {
	Name        string
	URL         string
	HasChildren bool
}
