// Package queries contains every SQL statement used by the Customizer.
package queries

// LoadContacts loads full master_contact rows; every column can be used as
// a {{variable}} in templates. Executed as the read-only audience role.
const LoadContacts = `SELECT * FROM master_contact WHERE id = ANY($1::bigint[])`
