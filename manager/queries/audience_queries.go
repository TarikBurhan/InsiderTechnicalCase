package queries

import "fmt"

// StreamAudience resolves an (already validated) audience SQL into
// contact ids and channel addresses. Contacts without an address for the
// channel or inactive contacts are skipped. Executed as the read-only
// audience role.
func StreamAudience(audienceSQL, addressColumn string) string {
	return fmt.Sprintf(`
SELECT mc.id, mc.%[2]s
FROM master_contact mc
WHERE mc.id IN (SELECT audience.id FROM (%[1]s) AS audience)
  AND mc.is_active
  AND mc.%[2]s IS NOT NULL AND mc.%[2]s <> ''
ORDER BY mc.id`, audienceSQL, addressColumn)
}

// SetStatementTimeout sets a transaction scoped statement timeout.
func SetStatementTimeout(ms int64) string {
	return fmt.Sprintf(`SET LOCAL statement_timeout = %d`, ms)
}
