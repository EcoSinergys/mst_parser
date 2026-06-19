import json
d = json.load(open('catalog_structured.json'))
imgs = sum(len(p['images']) for c in d['categories'] for s in c['subcategories'] for p in s['products'])
prods = sum(len(s['products']) for c in d['categories'] for s in c['subcategories'])
print(f'Товаров: {prods}, Изображений: {imgs}, Среднее: {imgs/prods:.1f} на товар')