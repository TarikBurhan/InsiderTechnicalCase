package validation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"notifier/panel/dto"
	"notifier/panel/request"
)

// MasterContactTable is the base audience table.
const MasterContactTable = "master_contact"

const maxInValues = 1000

var (
	identPattern      = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
	comparisonSQLOps  = map[string]string{"eq": "=", "neq": "<>", "gt": ">", "gte": ">=", "lt": "<", "lte": "<=", "like": "LIKE", "ilike": "ILIKE"}
	supportedOperator = "eq, neq, gt, gte, lt, lte, like, ilike, in, not_in, between, is_null, is_not_null"
)

// BuildFilterSQL turns a FILTER audience into a SELECT returning master
// contact ids. Every table / column is checked against the schema whitelist
// and every value is rendered as an escaped literal, so the resulting SQL is
// safe to store and execute later by the Manager.
func BuildFilterSQL(f request.AudienceFilter, schema dto.TableSchema) (string, error) {
	var errs Errors
	joinTable := strings.ToLower(strings.TrimSpace(f.Table))
	if joinTable == MasterContactTable {
		joinTable = ""
	}
	if joinTable != "" {
		if _, ok := schema[joinTable]; !ok || !identPattern.MatchString(joinTable) {
			return "", New("audience.filter.table", fmt.Sprintf("table %q is not allowed for audiences", f.Table))
		}
		if !schema.HasColumn(joinTable, "contact_id") {
			return "", New("audience.filter.table", fmt.Sprintf("table %q has no contact_id column", f.Table))
		}
	}

	joiner := " AND "
	switch strings.ToLower(f.Match) {
	case "", "all":
	case "any":
		joiner = " OR "
	default:
		errs.Add("audience.filter.match", "match must be 'all' or 'any'")
	}

	conds := make([]string, 0, len(f.Conditions))
	for i, c := range f.Conditions {
		field := fmt.Sprintf("audience.filter.conditions[%d]", i)
		cond, err := buildCondition(c, joinTable, schema)
		if err != "" {
			errs.Add(field, err)
			continue
		}
		conds = append(conds, cond)
	}
	if len(errs) > 0 {
		return "", errs
	}

	var sb strings.Builder
	sb.WriteString("SELECT DISTINCT mc.id FROM master_contact mc")
	if joinTable != "" {
		sb.WriteString(fmt.Sprintf(" JOIN %s t ON t.contact_id = mc.id", quoteIdent(joinTable)))
	}
	sb.WriteString(" WHERE mc.is_active")
	if len(conds) > 0 {
		sb.WriteString(" AND (")
		sb.WriteString(strings.Join(conds, joiner))
		sb.WriteString(")")
	}
	return sb.String(), nil
}

func buildCondition(c request.AudienceCondition, joinTable string, schema dto.TableSchema) (string, string) {
	table, column, errMsg := resolveField(c.Field, joinTable, schema)
	if errMsg != "" {
		return "", errMsg
	}
	alias := "mc"
	if table != MasterContactTable {
		alias = "t"
	}
	ref := alias + "." + quoteIdent(column)
	op := strings.ToLower(strings.TrimSpace(c.Operator))

	if sqlOp, ok := comparisonSQLOps[op]; ok {
		lit, err := literal(c.Value)
		if err != "" {
			return "", err
		}
		return fmt.Sprintf("%s %s %s", ref, sqlOp, lit), ""
	}
	switch op {
	case "is_null":
		return ref + " IS NULL", ""
	case "is_not_null":
		return ref + " IS NOT NULL", ""
	case "in", "not_in":
		values, ok := c.Value.([]any)
		if !ok || len(values) == 0 {
			return "", "value must be a non-empty array for " + op
		}
		if len(values) > maxInValues {
			return "", fmt.Sprintf("at most %d values allowed", maxInValues)
		}
		lits := make([]string, 0, len(values))
		for _, v := range values {
			lit, err := literal(v)
			if err != "" {
				return "", err
			}
			lits = append(lits, lit)
		}
		sqlOp := "IN"
		if op == "not_in" {
			sqlOp = "NOT IN"
		}
		return fmt.Sprintf("%s %s (%s)", ref, sqlOp, strings.Join(lits, ", ")), ""
	case "between":
		values, ok := c.Value.([]any)
		if !ok || len(values) != 2 {
			return "", "value must be an array of two items for between"
		}
		lo, err := literal(values[0])
		if err != "" {
			return "", err
		}
		hi, err := literal(values[1])
		if err != "" {
			return "", err
		}
		return fmt.Sprintf("%s BETWEEN %s AND %s", ref, lo, hi), ""
	}
	return "", fmt.Sprintf("unsupported operator %q (supported: %s)", c.Operator, supportedOperator)
}

func resolveField(field, joinTable string, schema dto.TableSchema) (string, string, string) {
	field = strings.ToLower(strings.TrimSpace(field))
	table, column := MasterContactTable, field
	if parts := strings.Split(field, "."); len(parts) == 2 {
		table, column = parts[0], parts[1]
	} else if len(parts) > 2 {
		return "", "", "invalid field " + field
	}
	if !identPattern.MatchString(table) || !identPattern.MatchString(column) {
		return "", "", fmt.Sprintf("invalid field %q", field)
	}
	if table != MasterContactTable && table != joinTable {
		return "", "", fmt.Sprintf("table %q must be set as filter.table to be used", table)
	}
	if !schema.HasColumn(table, column) {
		return "", "", fmt.Sprintf("unknown column %s.%s", table, column)
	}
	return table, column, ""
}

func literal(v any) (string, string) {
	switch val := v.(type) {
	case string:
		if strings.ContainsRune(val, 0) {
			return "", "value contains invalid characters"
		}
		return QuoteLiteral(val), ""
	case float64:
		return QuoteLiteral(strconv.FormatFloat(val, 'f', -1, 64)), ""
	case int:
		return QuoteLiteral(strconv.Itoa(val)), ""
	case int64:
		return QuoteLiteral(strconv.FormatInt(val, 10)), ""
	case bool:
		return QuoteLiteral(strconv.FormatBool(val)), ""
	case nil:
		return "", "value is required (use is_null for NULL checks)"
	default:
		return "", fmt.Sprintf("unsupported value type %T", v)
	}
}

// QuoteLiteral renders s as a SQL string literal (standard_conforming_strings
// is on, so doubling single quotes is sufficient).
func QuoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
