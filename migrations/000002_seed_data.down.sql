DELETE FROM audience_groups WHERE name = 'istanbul-gold';
DELETE FROM push_content WHERE name IN ('breaking-news', 'cart-reminder');
DELETE FROM email_content WHERE name IN ('weekly-newsletter', 'loyalty-email');
DELETE FROM sms_content WHERE name IN ('flash-sale-sms', 'welcome-sms');
DELETE FROM contact_purchases;
DELETE FROM master_contact;
