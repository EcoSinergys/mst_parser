package main

// ImageSet содержит ссылки на изображения продукта
type ImageSet struct {
	SmallRemoteURL string `json:"small_remote_url"`
	LargeRemoteURL string `json:"large_remote_url"`
	DestPath       string `json:"dest_path,omitempty"`        // целевой путь для MODX
	MenuIndex      int    `json:"menuindex,omitempty"`        // порядок в галерее
	LocalSmallPath string `json:"local_small_path,omitempty"` // оставлено для совместимости
	LocalLargePath string `json:"local_large_path,omitempty"`
}

// Product содержит полную информацию о продукте
type Product struct {
	ProductID      string            `json:"product_id"`
	Title          string            `json:"title"`
	URL            string            `json:"url"`
	Description    string            `json:"description"`
	Specifications map[string]string `json:"specifications"`
	TV             ProductTV         `json:"tv"`
	Images         []ImageSet        `json:"images"`

	MenuIndex int `json:"menuindex,omitempty"` // порядок в подкатегории

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
	Name      string    `json:"subcategory_name"`
	URL       string    `json:"subcategory_url"`
	Slug      string    `json:"slug,omitempty"`      // slug для URL и папки
	Image     string    `json:"image,omitempty"`     // изображение подкатегории
	MenuIndex int       `json:"menuindex,omitempty"` // порядок в категории
	Products  []Product `json:"products"`
}

// Category содержит категорию с подкатегориями
type Category struct {
	Name          string        `json:"category_name"`
	URL           string        `json:"category_url"`
	Slug          string        `json:"slug,omitempty"`      // slug для URL и папки
	Image         string        `json:"image,omitempty"`     // изображение категории
	MenuIndex     int           `json:"menuindex,omitempty"` // порядок в каталоге
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
	MenuIndex       int               `json:"menuindex"`
	ProductImage    string            `json:"product_image"`
	ProductCategory string            `json:"product_category"`
	SourceURL       string            `json:"source_url"`
	Specifications  map[string]string `json:"specifications,omitempty"`
	TV              map[string]string `json:"tv"`
	Images          []MODXImage       `json:"images,omitempty"`
}

// ProductTV содержит TV-параметры MODX Revolution 3 для товара
type ProductTV struct {
	Category     string `json:"category"`
	HeadM        string `json:"head_m"`
	PowerKW      string `json:"power_kw"`
	FlowM3H      string `json:"flow_m3h"`
	WeightKG     string `json:"weight_kg"`
	MaterialBody string `json:"material_body"`
}

// MODXImage — изображение для MODX с правильным путём
type MODXImage struct {
	Src       string `json:"src"`
	Alt       string `json:"alt,omitempty"`
	MenuIndex int    `json:"menuindex,omitempty"`
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

// slugify конвертирует название в slug для путей
func slugify(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			result = append(result, c+32) // to lower
		} else if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' {
			result = append(result, c)
		} else if c == ' ' || c == '_' {
			if len(result) == 0 || result[len(result)-1] != '-' {
				result = append(result, '-')
			}
		}
	}
	// remove trailing dash
	for len(result) > 0 && result[len(result)-1] == '-' {
		result = result[:len(result)-1]
	}
	return string(result)
}
