import json, os
from collections import Counter

with open('catalog_structured.json', 'r', encoding='utf-8') as f:
    catalog = json.load(f)

total_products = 0
total_images = 0
product_image_counts = []
categories = []

for cat in catalog['categories']:
    cat_name = cat['category_name']
    cat_products = 0
    cat_images = 0
    for sub in cat['subcategories']:
        for prod in sub['products']:
            cat_products += 1
            imgs_list = prod.get('images')
            if imgs_list is None:
                imgs_list = []
            img_count = len(imgs_list)
            cat_images += img_count
            product_image_counts.append(img_count)
    categories.append((cat_name, cat_products, cat_images))
    total_products += cat_products
    total_images += cat_images

print('=== СТАТИСТИКА КАТАЛОГА ===')
print(f'Всего категорий: {len(catalog["categories"])}')
print(f'Всего товаров: {total_products}')
print(f'Всего изображений: {total_images}')
print(f'Среднее изображений на товар: {total_images/total_products:.1f}')
print()

print('=== ПО КАТЕГОРИЯМ ===')
for name, prods, imgs in sorted(categories, key=lambda x: -x[1]):
    avg = imgs/prods if prods > 0 else 0
    print(f'{name:25s} | товаров: {prods:4d} | изображений: {imgs:5d} | среднее: {avg:.1f}')

print()
print('=== РАСПРЕДЕЛЕНИЕ КОЛИЧЕСТВА ИЗОБРАЖЕНИЙ ===')
counts = Counter(product_image_counts)
for cnt in sorted(counts.keys()):
    print(f'{cnt:2d} изображений: {counts[cnt]:4d} товаров')

print()
print('=== ПРИМЕР ЭЛЕМЕНТА ImageSet ===')
for cat in catalog['categories']:
    for sub in cat['subcategories']:
        for prod in sub['products']:
            if prod.get('images'):
                print(json.dumps(prod['images'][0], indent=2, ensure_ascii=False))
                break
        break
    break

print()
print('=== ТОВАРЫ С МАКСИМАЛЬНЫМ КОЛИЧЕСТВОМ ИЗОБРАЖЕНИЙ ===')
prod_list = []
for cat in catalog['categories']:
    for sub in cat['subcategories']:
        for prod in sub['products']:
            imgs_list = prod.get('images')
            if imgs_list is None:
                imgs_list = []
            prod_list.append((prod.get('title','?'), len(imgs_list)))

prod_list.sort(key=lambda x: -x[1])
for title, count in prod_list[:10]:
    print(f'{count:3d} изображений: {title[:60]}')