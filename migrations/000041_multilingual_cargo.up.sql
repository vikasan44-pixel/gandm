ALTER TABLE cargo_requests
    ADD COLUMN category text NOT NULL DEFAULT 'other',
    ADD COLUMN origin_labels jsonb,
    ADD COLUMN destination_labels jsonb,
    ADD CONSTRAINT cargo_requests_category_check CHECK (category IN (
        'chemicals', 'equipment', 'building_materials', 'home_appliances',
        'furniture', 'food', 'textiles', 'auto_parts', 'metals', 'timber',
        'medical_goods', 'agricultural_goods', 'plastics', 'dangerous_goods', 'other'
    ));

UPDATE cargo_requests
SET category = CASE
    WHEN description ~* 'мебел|furniture|家具' THEN 'furniture'
    WHEN description ~* 'хим|chemical|化工' THEN 'chemicals'
    WHEN description ~* 'оборуд|equipment|设备' THEN 'equipment'
    WHEN description ~* 'строй|строител|building material|建材' THEN 'building_materials'
    WHEN description ~* 'бытов.*тех|appliance|家电' THEN 'home_appliances'
    WHEN description ~* 'продукт|пищ|food|食品' THEN 'food'
    WHEN description ~* 'текстил|textile|纺织' THEN 'textiles'
    WHEN description ~* 'автозап|auto part|汽车配件' THEN 'auto_parts'
    WHEN description ~* 'металл|metal|金属' THEN 'metals'
    WHEN description ~* 'древес|лесомат|timber|wood|木材' THEN 'timber'
    WHEN description ~* 'медицин|medical|医疗' THEN 'medical_goods'
    WHEN description ~* 'сельхоз|agricultur|农产品' THEN 'agricultural_goods'
    WHEN description ~* 'пластик|plastic|塑料' THEN 'plastics'
    WHEN description ~* 'опасн|dangerous|危险品' THEN 'dangerous_goods'
    ELSE 'other'
END;

ALTER TABLE participant_routes
    ADD COLUMN origin_labels jsonb,
    ADD COLUMN destination_labels jsonb;

ALTER TABLE consolidated_requests
    ADD COLUMN origin_labels jsonb,
    ADD COLUMN destination_labels jsonb;

