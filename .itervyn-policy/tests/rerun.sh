#!/usr/bin/env bash
set -euo pipefail
source=$(cd "$1" && pwd -P)
binaries=$(cd "$2" && pwd -P)
launcher=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)/run-gate.sh
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
git clone --quiet --no-hardlinks "$source" "$scratch/candidate"
candidate="$scratch/candidate"
revision=$(git -C "$candidate" rev-parse HEAD)
exclude="$candidate/.git/info/exclude"

for state in present missing; do
  if [[ $state == present ]]; then
    printf '# preserved bytes\nexisting-pattern' > "$exclude"
    cp "$exclude" "$scratch/original"
  else
    rm -f "$exclude"
  fi
  for attempt in 1 2; do
    output="$scratch/$state-$attempt"
    mkdir "$output"
    bash "$launcher" "$candidate" "$revision" "$binaries" "$output"
    if [[ $state == present ]]; then
      cmp "$scratch/original" "$exclude"
    else
      test ! -e "$exclude"
    fi
  done
done
