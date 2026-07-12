#!/bin/sh
set -eu

script_dir=${0%/*}
[ "$script_dir" != "$0" ] || script_dir=.
script_dir=$(CDPATH='' cd -- "$script_dir" && pwd -P) || exit 1

exec /usr/bin/env -u BASH_ENV -u ENV -u CDPATH \
  /bin/bash --noprofile --norc \
  "$script_dir/verify-implementation-receipt.bash" "$@"
