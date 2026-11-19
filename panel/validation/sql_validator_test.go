package validation

import (
	"strings"
	"testing"
)

func TestSanitizeAudienceSQL_Valid(t *testing.T) {
	cases := map[string]string{
		"simple":            "SELECT id FROM master_contact WHERE city = 'Istanbul'",
		"trailing semi":     "SELECT id FROM master_contact;",
		"cte":               "WITH x AS (SELECT id FROM master_contact) SELECT id FROM x",
		"keyword in string": "SELECT id FROM master_contact WHERE city = 'drop; update -- me'",
		"column like kw":    "SELECT id FROM master_contact WHERE updated_at > now() - interval '1 day'",
		"join":              "SELECT mc.id FROM master_contact mc JOIN contact_purchases p ON p.contact_id = mc.id WHERE p.amount > 100",
	}
	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := SanitizeAudienceSQL(sql)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.HasSuffix(got, ";") {
				t.Fatalf("trailing semicolon not stripped: %q", got)
			}
		})
	}
}

func TestSanitizeAudienceSQL_Invalid(t *testing.T) {
	cases := map[string]string{
		"empty":            "   ",
		"delete":           "DELETE FROM master_contact",
		"multi statement":  "SELECT id FROM master_contact; DROP TABLE notifications",
		"comment":          "SELECT id FROM master_contact -- hi",
		"block comment":    "SELECT id FROM master_contact /* x */",
		"select into":      "SELECT id INTO tmp FROM master_contact",
		"other table only": "SELECT id FROM notifications",
		"pg_sleep":         "SELECT id FROM master_contact WHERE pg_sleep(10) IS NOT NULL",
		"dollar quote":     "SELECT id FROM master_contact WHERE city = $$x$$",
		"update in cte":    "WITH u AS (UPDATE master_contact SET city='x' RETURNING id) SELECT id FROM u",
		"for update":       "SELECT id FROM master_contact FOR UPDATE",
		"set_config":       "SELECT id FROM master_contact WHERE set_config('a','b',false) = 'b'",
		"unterminated":     "SELECT id FROM master_contact WHERE city = 'abc",
	}
	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := SanitizeAudienceSQL(sql); err == nil {
				t.Fatalf("expected error for %q", sql)
			}
		})
	}
}
