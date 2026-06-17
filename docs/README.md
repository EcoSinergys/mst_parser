# MST Pumps Catalog Parser v2.0

Парсер каталога насосов с сайта [mstpumps.com](https://www.mstpumps.com) с экспортом в JSON для CMS MODX 3.

## Возможности

- ✅ Сбор **963+ товаров** со всех страниц каталога
- ✅ Парсинг по категориям и подкатегориям
- ✅ Извлечение: название, описание, характеристики, изображения
- ✅ Автоматическое скачивание изображений (small + large)
- ✅ Защита от блокировки: ротация User-Agent, случайные задержки, retry
- ✅ Экспорт: `catalog_structured.json` + `modx_import.json`

## Быстрый старт

### 1. Сборка

```bash
go build -o mst_parser.exe .
```

### 2. Запуск

**Полный парсинг всех товаров (режим A):**
```bash
mst_parser.exe --mode=A
```

**Парсинг одной категории (режим B):**
```bash
mst_parser.exe --mode=B --category="https://www.mstpumps.com/slurry-pumps/"
```

**Тест без изображений:**
```bash
mst_parser.exe --mode=A --limit=5 --skip-images
```

### 3. Параметры

| Флаг | Описание |
|------|----------|
| `--mode=A/B` | Режим: A — productlist, B — по категориям |
| `--category=URL` | URL конкретной категории (режим B) |
| `--limit=N` | Ограничить количество продуктов |
| `--skip-images` | Не скачивать изображения |

## Структура проекта

```
mst_parser/
├── main.go          # Точка входа
├── client.go        # HTTP-клиент с защитой
├── catalog.go       # Парсинг категорий
├── product.go       # Парсинг товаров
├── downloader.go    # Скачивание изображений
├── storage.go       # Сохранение JSON
├── types.go         # Структуры данных
├── main_test.go     # Unit-тесты
├── ТЗ.md            # Техническое задание
├── README.md        # Эта инструкция
└── .gitignore       # Исключения Git
```

## Выходные файлы

| Файл | Описание |
|------|----------|
| `catalog_structured.json` | Иерархический каталог |
| `modx_import.json` | Плоский список для MODX 3 |
| `downloaded_images/small/` | Маленькие изображения |
| `downloaded_images/large/` | Большие изображения |

## Тестирование

```bash
# Все тесты
go test -v ./...

# Конкретный тест
go test -v -run TestParseProductLinks
```

## Системные требования

- Go 1.25+
- Windows 10/11
- Интернет-соединение