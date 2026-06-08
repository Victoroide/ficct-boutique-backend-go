#!/bin/sh
set -eu

if [ "$#" -gt 0 ] && [ "$1" != "server" ]; then
  exec "$@"
fi

echo "running migrations"
migrate up
echo "migrations complete"

echo "running seed"
seed
echo "seed complete"

echo "starting server"
exec server
