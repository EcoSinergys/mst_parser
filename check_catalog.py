import json

with open('catalog_structured.json', encoding='utf-8') as f:
    d = json.load(f)

print(f'Категорий: {len(d["categories"])}')

prods = 0
imgs = 0
none_subs = 0

for c in d['categories']:
    cat_name = c.get('name', c.get('title', '?'))
    print(f'\n  📁 {cat_name}')
    for s in c['subcategories']:
        if s.get('products') is None:
            sub_name = s.get('name', s.get('title', '?'))
            print(f'    ⚠️ {sub_name} - products=None')
            none_subs += 1
            continue
        prods += len(s['products'])
        for p in s['products']:
            if p is None:
                continue
            imgs += len(p.get('images', []) or [])
    print(f'    Подкатегорий: {len(c["subcategories"])}')

print(f'\n{"="*40}')
print(f'Всего продуктов: {prods}')
print(f'Всего изображений: {imgs}')
print(f'Подкатегорий с None: {none_subs}')
if prods > 0:
    print(f'В среднем: {imgs/prods:.1f} на товар')
