#!/usr/bin/env bash

set -e

host="$1"
shift
cmd="$@"

until curl -s "$host" > /dev/null; do
  >&2 echo "MinIO is unavailable - sleeping"
  sleep 1
done

>&2 echo "MinIO is up - executing command"
exec $cmd