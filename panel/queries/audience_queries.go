package queries

import "fmt"

// CountAudience wraps a validated audience SQL and counts how many of the
// returned ids really exist in master_contact. It fails when the query does
// not expose an `id` column comparable to master_contact.id. It is executed
// as the read-only audience role inside a READ ONLY transaction.
func CountAudience(audienceSQL string) string {
	return fmt.Sprintf(`SELECT count(DISTINCT mc.id) FROM (%s) AS audience JOIN master_contact mc ON mc.id = audience.id`, audienceSQL)
}

// SetStatementTimeout limits how long audience validation may run.
func SetStatementTimeout(ms int64) string {
	return fmt.Sprintf(`SET LOCAL statement_timeout = %d`, ms)
}

// TableColumns lists the columns of whitelisted tables.
const TableColumns = `
SELECT table_name, column_name, data_type
FROM information_schema.columns
WHERE table_schema = 'public' AND table_name = ANY($1)
ORDER BY table_name, ordinal_position`

const audienceGroupColumns = `id, name, description, source_type, definition, sql_query, contact_count, validated_at, created_at, updated_at`

// InsertAudienceGroup saves a validated audience group.
const InsertAudienceGroup = `
INSERT INTO audience_groups (name, description, source_type, definition, sql_query, contact_count)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING ` + audienceGroupColumns

// GetAudienceGroup loads an audience group.
const GetAudienceGroup = `SELECT ` + audienceGroupColumns + ` FROM audience_groups WHERE id = $1`

// ListAudienceGroups pages through audience groups.
const ListAudienceGroups = `SELECT ` + audienceGroupColumns + ` FROM audience_groups ORDER BY id DESC LIMIT $1 OFFSET $2`

// CountAudienceGroups counts audience groups.
const CountAudienceGroups = `SELECT count(*) FROM audience_groups`

// DeleteAudienceGroup deletes an audience group.
const DeleteAudienceGroup = `DELETE FROM audience_groups WHERE id = $1`
