#!/usr/bin/env bash
# Inputs are a disposable checkout without active processes, trusted Linux
# binaries, and a fresh external evidence directory, all supplied by CI.
set -euo pipefail
umask 077

candidate=$(cd "$1" && pwd -P)
revision=$2
binaries=$(cd "$3" && pwd -P)
evidence=$(cd "$4" && pwd -P)
policy=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
image='buildpack-deps@sha256:5bac6b3020afcf0358de87990605d5dc4c3f060629381912266b9b9278375af3'

[[ $revision =~ ^([0-9a-f]{40}|[0-9a-f]{64})$ ]]
[[ -d "$candidate/.git" && ! -L "$candidate/.git" ]]
for external in "$binaries" "$evidence" "$policy"; do
  case "$external/" in
    "$candidate/"*) exit 2 ;;
  esac
done
test -z "$(find "$evidence" -mindepth 1 -maxdepth 1 -print -quit)"

directory="$candidate/.invaris"
stage="$directory/itervyn-harness.json"
exclude="$candidate/.git/info/exclude"
if [[ -L "$directory" || ( -e "$directory" && ! -d "$directory" ) ||
      -e "$stage" || -L "$stage" || -L "$exclude" ]]; then
  echo 'Reserved harness staging path is unavailable.' >&2
  exit 2
fi
# A tracked path stays part of the candidate even if the file is absent.
tracked=$(env -i PATH=/usr/bin:/bin GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null \
  /usr/bin/git -C "$candidate" -c core.fsmonitor=false ls-files -- .invaris/itervyn-harness.json)
if [[ -n "$tracked" ]]; then
  echo 'Reserved harness staging path is tracked.' >&2
  exit 2
fi
exclude_backup=$(mktemp "${TMPDIR:-/tmp}/itervyn-exclude.XXXXXX")
exclude_existed=false
if [[ -e "$exclude" ]]; then
  cp -p "$exclude" "$exclude_backup"
  exclude_existed=true
fi
cleanup() {
  rm -f "$stage"
  rmdir "$directory" 2>/dev/null || true
  if [[ $exclude_existed == true ]]; then
    cp -p "$exclude_backup" "$exclude"
  else
    rm -f "$exclude"
  fi
  rm -f "$exclude_backup"
}
trap cleanup EXIT
mkdir -p "$directory"
install -m 0444 "$policy/invaris.json" "$stage"
printf '\n/.invaris/itervyn-harness.json\n' >> "$exclude"

# No candidate commands run. The trusted guard reads candidate source through
# a read-only mount. Only the trusted container processes write the evidence.
docker run --rm --read-only --network none --cap-drop ALL \
  --security-opt no-new-privileges --pids-limit 128 --memory 512m \
  --user "$(id -u):$(id -g)" --tmpfs /tmp:rw,nosuid,nodev,noexec,mode=1777 \
  --mount "type=bind,source=$candidate,target=/candidate,readonly" \
  --mount "type=bind,source=$policy,target=/trusted/policy,readonly" \
  --mount "type=bind,source=$binaries,target=/trusted/bin,readonly" \
  --mount "type=bind,source=$evidence,target=/evidence" \
  --workdir /candidate --entrypoint /usr/bin/env "$image" \
  -i PATH=/usr/bin:/bin HOME=/tmp TMPDIR=/tmp LANG=C.UTF-8 \
  /trusted/bin/itervyn verify --config /trusted/policy/itervyn.json \
  --expect-revision "$revision" --format json > "$evidence/result.json"
