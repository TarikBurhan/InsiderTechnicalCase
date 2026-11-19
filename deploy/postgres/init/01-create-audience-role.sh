#!/bin/sh
# Creates the read-only role used to execute user supplied audience SQL.
# Runs once, on the first start of the postgres container.
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-SQL
    DO \$\$
    BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'audience_reader') THEN
            CREATE ROLE audience_reader LOGIN PASSWORD '${AUDIENCE_DB_PASSWORD}';
        END IF;
    END
    \$\$;
    ALTER ROLE audience_reader SET default_transaction_read_only = on;
    ALTER ROLE audience_reader SET statement_timeout = '60s';
    GRANT USAGE ON SCHEMA public TO audience_reader;
    GRANT CONNECT ON DATABASE "${POSTGRES_DB}" TO audience_reader;
SQL
