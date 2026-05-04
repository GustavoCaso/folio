#!/bin/sh
set -e

PUID=${PUID:-1000}
PGID=${PGID:-1000}

echo "Starting with PUID=$PUID, PGID=$PGID"

if [ "$(id -u folio)" != "$PUID" ] || [ "$(id -g folio)" != "$PGID" ]; then
    echo "Adjusting user and group IDs..."
    groupmod -g "$PGID" folio
    usermod -u "$PUID" folio
fi

echo "Fixing ownership of ${DATA_DIR:-/data}..."
chown -R "$PUID:$PGID" "${DATA_DIR:-/data}"

exec su-exec folio "$@"
