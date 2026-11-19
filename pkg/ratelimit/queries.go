package ratelimit

// AcquireTokensQuery calls the acquire_rate_tokens() function created by the
// migrations. It atomically reserves up to $2 tokens in the current one
// second window (database clock) for limiter key $1 with capacity $3, and
// returns how many were granted.
const AcquireTokensQuery = `SELECT acquire_rate_tokens($1, $2, $3)`
