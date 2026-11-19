package queries

import "fmt"

// StreamAudience resolves an (already validated) audience SQL into
// contact ids and channel addresses. Contacts without an address for the
// channel or inactive contacts are skipped. Every address is returned once:
// when several contacts share a phone / email / device token, the contact
// with the lowest id is used (emails are compared case-insensitively).
// Executed as the read-only audience role.
func StreamAudience(audienceSQL, addressColumn, addressKeyExpr string) string {
	return fmt.Sprintf(`
SELECT DISTINCT ON (c.address_key) c.id, c.address
FROM (
	SELECT mc.id, btrim(mc.%[2]s) AS address, %[3]s AS address_key
	FROM master_contact mc
	WHERE mc.id IN (SELECT audience.id FROM (%[1]s) AS audience)
	  AND mc.is_active
	  AND mc.%[2]s IS NOT NULL AND btrim(mc.%[2]s) <> ''
) AS c
ORDER BY c.address_key, c.id`, audienceSQL, addressColumn, addressKeyExpr)
}

// SetStatementTimeout sets a transaction scoped statement timeout.
func SetStatementTimeout(ms int64) string {
	return fmt.Sprintf(`SET LOCAL statement_timeout = %d`, ms)
}
