#!/bin/sh
set -e

PUID=${PUID:-1000}
PGID=${PGID:-1000}

echo "Starting with PUID=$PUID, PGID=$PGID"

if [ "$(id -u folio)" != "$PUID" ] || [ "$(id -g folio)" != "$PGID" ]; then
    echo "Adjusting user and group IDs..."
    if ! groupmod -g "$PGID" folio 2>/dev/null; then
        echo "Warning: Could not modify group; continuing anyway"
    fi
    if ! usermod -u "$PUID" folio 2>/dev/null; then
        echo "Warning: Could not modify user; continuing anyway"
    fi
    
    echo "Fixing ownership of ${DATA_DIR:-/data}..."
    if ! chown -R "$PUID:$PGID" "${DATA_DIR:-/data}" 2>/dev/null; then
        echo "Warning: Could not chown DATA_DIR; continuing anyway"
    fi
fi

exec su-exec folio "$@"
