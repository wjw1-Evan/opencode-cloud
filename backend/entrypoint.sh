#!/bin/bash
set -e

# Initialize PostgreSQL data directory if empty
if [ ! -f /var/lib/postgresql/data/PG_VERSION ]; then
    echo "Initializing PostgreSQL database..."
    su-exec postgres /usr/lib/postgresql/17/bin/initdb \
        -D /var/lib/postgresql/data \
        --encoding=UTF-8 \
        --lc-collate=C \
        --lc-ctype=C

    # Start PostgreSQL temporarily to create user and database
    su-exec postgres /usr/lib/postgresql/17/bin/pg_ctl \
        -D /var/lib/postgresql/data \
        -w start

    # Create user and database
    su-exec postgres psql -c "CREATE USER ${POSTGRES_USER} WITH SUPERUSER PASSWORD '${POSTGRES_PASSWORD}';"
    su-exec postgres psql -c "CREATE DATABASE ${POSTGRES_DB} OWNER ${POSTGRES_USER};"
    su-exec postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE ${POSTGRES_DB} TO ${POSTGRES_USER};"

    # Stop PostgreSQL
    su-exec postgres /usr/lib/postgresql/17/bin/pg_ctl \
        -D /var/lib/postgresql/data \
        -m fast \
        -w stop

    echo "PostgreSQL initialization complete."
fi

# Ensure correct ownership
chown -R postgres:postgres /var/lib/postgresql/data

# Start all services via supervisord
exec /usr/bin/supervisord -c /etc/supervisord.conf
