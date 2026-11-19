package validation

import (
	"strings"
	"testing"

	"notifier/panel/dto"
	"notifier/panel/request"
)

func testSchema() dto.TableSchema {
	return dto.TableSchema{
		"master_contact":    {"id": "bigint", "city": "text", "loyalty_tier": "text", "birth_date": "date", "is_active": "boolean"},
		"contact_purchases": {"contact_id": "bigint", "amount": "numeric", "product_category": "text"},
	}
}

func TestBuildFilterSQL_MasterContact(t *testing.T) {
	f := request.AudienceFilter{Conditions: []request.AudienceCondition{
		{Field: "city", Operator: "eq", Value: "Istanbul"},
		{Field: "loyalty_tier", Operator: "in", Value: []any{"gold", "platinum"}},
	}}
	sql, err := BuildFilterSQL(f, testSchema())
	if err != nil {
		t.Fatal(err)
	}
	want := `SELECT DISTINCT mc.id FROM master_contact mc WHERE mc.is_active AND (mc."city" = 'Istanbul' AND mc."loyalty_tier" IN ('gold', 'platinum'))`
	if sql != want {
		t.Fatalf("got  %s\nwant %s", sql, want)
	}
}

func TestBuildFilterSQL_JoinAndAny(t *testing.T) {
	f := request.AudienceFilter{Table: "contact_purchases", Match: "any", Conditions: []request.AudienceCondition{
		{Field: "contact_purchases.amount", Operator: "gte", Value: float64(500)},
		{Field: "city", Operator: "is_null"},
	}}
	sql, err := BuildFilterSQL(f, testSchema())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `JOIN "contact_purchases" t ON t.contact_id = mc.id`) ||
		!strings.Contains(sql, `t."amount" >= '500' OR mc."city" IS NULL`) {
		t.Fatalf("unexpected sql: %s", sql)
	}
}

func TestBuildFilterSQL_EscapesValues(t *testing.T) {
	f := request.AudienceFilter{Conditions: []request.AudienceCondition{
		{Field: "city", Operator: "eq", Value: "x' OR '1'='1"},
	}}
	sql, err := BuildFilterSQL(f, testSchema())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `'x'' OR ''1''=''1'`) {
		t.Fatalf("value not escaped: %s", sql)
	}
}

func TestBuildFilterSQL_Rejects(t *testing.T) {
	cases := map[string]request.AudienceFilter{
		"unknown column":    {Conditions: []request.AudienceCondition{{Field: "password", Operator: "eq", Value: "x"}}},
		"injection field":   {Conditions: []request.AudienceCondition{{Field: "city; drop", Operator: "eq", Value: "x"}}},
		"bad operator":      {Conditions: []request.AudienceCondition{{Field: "city", Operator: "regex", Value: "x"}}},
		"table not joined":  {Conditions: []request.AudienceCondition{{Field: "contact_purchases.amount", Operator: "gt", Value: 1.0}}},
		"table not allowed": {Table: "notifications", Conditions: []request.AudienceCondition{}},
		"in not array":      {Conditions: []request.AudienceCondition{{Field: "city", Operator: "in", Value: "x"}}},
		"between bad":       {Conditions: []request.AudienceCondition{{Field: "birth_date", Operator: "between", Value: []any{"2000-01-01"}}}},
		"nil value":         {Conditions: []request.AudienceCondition{{Field: "city", Operator: "eq"}}},
		"bad match":         {Match: "xor", Conditions: []request.AudienceCondition{{Field: "city", Operator: "eq", Value: "x"}}},
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := BuildFilterSQL(f, testSchema()); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
