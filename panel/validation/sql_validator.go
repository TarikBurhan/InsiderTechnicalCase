package validation

import (
	"regexp"
	"strings"
)

// MaxAudienceSQLLength bounds the size of user supplied audience SQL.
const MaxAudienceSQLLength = 10000

var (
	stringLiteralPattern = regexp.MustCompile(`'(?:[^']|'')*'`)
	quotedIdentPattern   = regexp.MustCompile(`"(?:[^"]|"")*"`)
	startsWithSelect     = regexp.MustCompile(`^(select|with)\b`)
	forbiddenKeywords    = regexp.MustCompile(`\b(insert|update|delete|drop|alter|create|truncate|grant|revoke|copy|execute|call|do|lock|vacuum|analyze|cluster|reindex|comment|merge|refresh|listen|notify|unlisten|prepare|deallocate|set|reset|into|returning|discard|checkpoint|security|owner)\b`)
	forbiddenFunctions   = regexp.MustCompile(`\b(pg_\w+|lo_\w+|dblink\w*|set_config|current_setting|query_to_xml\w*|xpath\w*|txid_\w+)\s*\(`)
	referencesContacts   = regexp.MustCompile(`\bmaster_contact\b`)
)

// SanitizeAudienceSQL performs the static part of audience SQL validation.
// The query must be a single read-only SELECT (or WITH ... SELECT) statement
// that references master_contact. The dynamic part (actually running it as
// the read-only audience role and checking the returned ids exist in
// master_contact) is done by the audience repository.
func SanitizeAudienceSQL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	for strings.HasSuffix(s, ";") {
		s = strings.TrimSpace(strings.TrimSuffix(s, ";"))
	}
	switch {
	case s == "":
		return "", New("audience.sql", "sql is required")
	case len(s) > MaxAudienceSQLLength:
		return "", New("audience.sql", "sql is too long")
	case strings.ContainsRune(s, 0):
		return "", New("audience.sql", "sql contains invalid characters")
	}

	// Remove literal contents so keywords inside strings do not trigger rules,
	// while still detecting dangerous constructs outside of them.
	stripped := stringLiteralPattern.ReplaceAllString(s, "''")
	stripped = quotedIdentPattern.ReplaceAllString(stripped, `""`)
	lower := strings.ToLower(stripped)

	switch {
	case strings.Contains(lower, ";"):
		return "", New("audience.sql", "only a single statement is allowed")
	case strings.Contains(lower, "--") || strings.Contains(lower, "/*"):
		return "", New("audience.sql", "comments are not allowed")
	case strings.Contains(lower, "$"):
		return "", New("audience.sql", "dollar quoting / parameters are not allowed")
	case strings.Count(stripped, "'")%2 != 0:
		return "", New("audience.sql", "unterminated string literal")
	case !startsWithSelect.MatchString(lower):
		return "", New("audience.sql", "only SELECT statements are allowed")
	}
	if kw := forbiddenKeywords.FindString(lower); kw != "" {
		return "", New("audience.sql", "forbidden keyword: "+strings.ToUpper(kw))
	}
	if fn := forbiddenFunctions.FindString(lower); fn != "" {
		return "", New("audience.sql", "forbidden function: "+strings.TrimSpace(strings.TrimSuffix(fn, "(")))
	}
	if !referencesContacts.MatchString(lower) {
		return "", New("audience.sql", "query must select from master_contact")
	}
	return s, nil
}
