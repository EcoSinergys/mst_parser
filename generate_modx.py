import json

with open('catalog_structured.json', encoding='utf-8') as f:
    catalog = json.load(f)

modx_products = []

for cat in catalog.get('categories', []):
    for sub in cat.get('subcategories', []):
        products = sub.get('products') or []
        for p in products:
            if p is None:
                continue
            
            modx = {
                'pagetitle': p.get('title', ''),
                'alias': p.get('alias', ''),
                'parent': p.get('parent', 0),
                'template': p.get('template', 0),
                'published': p.get('published', True),
                'description': p.get('description', ''),
                'product_image': p.get('product_image', ''),
                'product_category': p.get('product_category', ''),
                'source_url': p.get('url', ''),
                'images': p.get('images', []),
            }
            
            # Сериализуем спецификации в JSON-строку
            specs = p.get('specifications', {}) or {}
            if specs:
                modx['specifications'] = json.dumps(specs, ensure_ascii=False)
            else:
                modx['specifications'] = ''
            
            modx_products.append(modx)

with open('modx_import.json', 'w', encoding='utf-8') as f:
    json.dump(modx_products, f, indent=2, ensure_ascii=False)

print(f'✅ modx_import.json создан! Продуктов: {len(modx_products)}')