-- =====================================================================
-- 000002 - demo data: contacts, purchases, contents and an audience group
-- =====================================================================

INSERT INTO master_contact (first_name, last_name, email, phone, device_token, city, country, language, gender, birth_date, loyalty_tier)
SELECT
    (ARRAY['Ahmet','Ayse','Mehmet','Zeynep','Can','Elif','Emre','Selin','Burak','Deniz'])[1 + (i % 10)],
    (ARRAY['Yilmaz','Kaya','Demir','Sahin','Celik','Yildiz','Aydin','Ozturk'])[1 + (i % 8)],
    'user' || i || '@example.com',
    '+90555' || lpad(i::text, 7, '0'),
    'device-token-' || md5(i::text),
    (ARRAY['Istanbul','Ankara','Izmir','Bursa','Antalya'])[1 + (i % 5)],
    'TR',
    (ARRAY['tr','en'])[1 + (i % 2)],
    (ARRAY['female','male'])[1 + (i % 2)],
    DATE '1970-01-01' + (i * 37 % 15000),
    (ARRAY['bronze','silver','gold','platinum'])[1 + (i % 4)]
FROM generate_series(1, 1000) AS s(i);

INSERT INTO contact_purchases (contact_id, product_category, amount, purchased_at)
SELECT
    1 + (i % 1000),
    (ARRAY['electronics','fashion','books','home','sports'])[1 + (i % 5)],
    round((10 + (i * 13 % 990))::numeric, 2),
    now() - ((i % 90) || ' days')::interval
FROM generate_series(1, 3000) AS s(i);

INSERT INTO sms_content (name, body, variables) VALUES
    ('flash-sale-sms', 'Flash sale! %50 indirim sadece bugun. Kacirma!', '{}'),
    ('welcome-sms', 'Merhaba {{first_name}}, {{city}} magazamiza hos geldin!', '{first_name,city}');

INSERT INTO email_content (name, subject, body, is_html, variables) VALUES
    ('weekly-newsletter', 'Haftalik bulten', '<h1>Bu haftanin firsatlari</h1><p>Yeni urunleri kesfet.</p>', TRUE, '{}'),
    ('loyalty-email', 'Merhaba {{first_name}}, {{loyalty_tier}} uyelere ozel!', '<p>Sevgili {{first_name}} {{last_name}}, sana ozel firsatlar seni bekliyor.</p>', TRUE, '{first_name,loyalty_tier,last_name}');

INSERT INTO push_content (name, title, body, image_url, data, variables) VALUES
    ('breaking-news', 'Son dakika', 'Onemli bir gelisme var, hemen goz at.', NULL, '{"deeplink":"app://news"}', '{}'),
    ('cart-reminder', '{{first_name}}, sepetin seni bekliyor', 'Sepetindeki urunler tukenmeden siparisini tamamla.', NULL, '{"deeplink":"app://cart"}', '{first_name}');

INSERT INTO audience_groups (name, description, source_type, definition, sql_query, contact_count)
SELECT 'istanbul-gold', 'Istanbul gold members', 'SQL', NULL,
       'SELECT id FROM master_contact WHERE city = ''Istanbul'' AND loyalty_tier = ''gold''',
       count(*)
FROM master_contact WHERE city = 'Istanbul' AND loyalty_tier = 'gold';
