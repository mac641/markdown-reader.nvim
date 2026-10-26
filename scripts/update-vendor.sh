#!/bin/sh
# Re-fetch a vendored C/C++ library at a given tag and copy the files we
# need into third_party/<name>/. The tarball hash and tag commit are
# recorded in third_party/VERSIONS; re-fetching a tag that is already
# recorded must reproduce the same hash.
#
# Go dependencies are pinned in go.mod/go.sum instead and updated with
# `go get <module>@<version> && go mod tidy`.
#
# Usage: scripts/update-vendor.sh webview <tag>
set -eu

if [ $# -ne 2 ]; then
  echo "usage: $0 webview <tag>" >&2
  exit 2
fi
name=$1
tag=$2

# Each entry is "<path in upstream tarball>:<path below third_party/<name>/>".
case $name in
  webview)
    repo=https://github.com/webview/webview
    files="core/include/webview.h:webview.h core/include/webview/webview.h:webview/webview.h core/src/webview.cc:webview.cc LICENSE:LICENSE"
    ;;
  *)
    echo "unknown library: $name" >&2
    exit 2
    ;;
esac

root=$(cd "$(dirname "$0")/.." && pwd)
versions=$root/third_party/VERSIONS
dest=$root/third_party/$name

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

curl -fsSL "$repo/archive/refs/tags/$tag.tar.gz" -o "$tmp/src.tar.gz"
hash=$(sha256 "$tmp/src.tar.gz")

# Annotated tags resolve via ^{}; lightweight tags only have the plain ref.
commit=$(git ls-remote "$repo" "refs/tags/$tag^{}" | cut -f1)
[ -n "$commit" ] || commit=$(git ls-remote "$repo" "refs/tags/$tag" | cut -f1)
[ -n "$commit" ] || { echo "tag $tag not found in $repo" >&2; exit 1; }

if [ -f "$versions" ]; then
  recorded=$(awk -v n="$name" -v t="tag=$tag" '$1 == n && $3 == t { sub("sha256=", "", $5); print $5 }' "$versions")
  if [ -n "$recorded" ] && [ "$recorded" != "$hash" ]; then
    echo "sha256 mismatch for $name $tag: recorded $recorded, got $hash" >&2
    exit 1
  fi
fi

mkdir -p "$tmp/x"
tar xzf "$tmp/src.tar.gz" -C "$tmp/x" --strip-components=1

rm -rf "$dest"
for pair in $files; do
  from=${pair%%:*}
  to=${pair#*:}
  mkdir -p "$(dirname "$dest/$to")"
  cp "$tmp/x/$from" "$dest/$to"
done

{
  echo "# name  repository  tag  commit  tarball-sha256 (managed by scripts/update-vendor.sh)"
  if [ -f "$versions" ]; then
    grep -v '^#' "$versions" | awk -v n="$name" '$1 != n'
  fi
  echo "$name $repo tag=$tag commit=$commit sha256=$hash"
} > "$tmp/VERSIONS"
{ head -n1 "$tmp/VERSIONS"; tail -n +2 "$tmp/VERSIONS" | sort; } > "$versions"

echo "vendored $name $tag ($commit)"
