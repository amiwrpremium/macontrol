#!/usr/bin/env sh
# macontrol — manual install bootstrap.
#
#   curl -fsSL https://raw.githubusercontent.com/amiwrpremium/macontrol/master/scripts/install.sh | sh
#
# Downloads the latest GitHub release asset for darwin/arm64, verifies the
# SHA-256 checksum, drops the binary in a PATH directory, and prints next
# steps. Refuses to run on non-Apple-Silicon Darwin.

set -eu

REPO="amiwrpremium/macontrol"
BIN="macontrol"

die() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

# --- platform check ---
os="$(uname -s)"
arch="$(uname -m)"
case "$os" in
  Darwin) ;;
  *) die "macontrol targets macOS. Got OS=$os." ;;
esac
case "$arch" in
  arm64) ;;
  *) die "macontrol targets Apple Silicon (arm64). Got arch=$arch." ;;
esac

need() { command -v "$1" >/dev/null 2>&1 || die "required: $1"; }
need curl
need shasum
need tar
need install

# --- pick install dir ---
if [ -w /usr/local/bin ]; then
  install_dir=/usr/local/bin
elif [ "$(id -u)" = "0" ]; then
  install_dir=/usr/local/bin
  mkdir -p "$install_dir"
else
  install_dir="$HOME/.local/bin"
  mkdir -p "$install_dir"
  case ":$PATH:" in
    *":$install_dir:"*) ;;
    # The literal $PATH in the printf format is intentional — we're
    # printing a snippet for the user to paste into their shellrc.
    # shellcheck disable=SC2016
    *) printf 'note: %s is not on PATH yet — add `export PATH="%s:$PATH"` to your shell rc.\n' "$install_dir" "$install_dir" ;;
  esac
fi

# --- latest release metadata ---
api="https://api.github.com/repos/$REPO/releases/latest"
tag="$(curl -fsSL "$api" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)"
[ -n "$tag" ] || die "could not resolve latest release tag from $api"
version="${tag#v}"

archive="${BIN}_${version}_darwin_arm64.tar.gz"
base="https://github.com/$REPO/releases/download/$tag"
archive_url="$base/$archive"
checksums_url="$base/checksums.txt"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

printf 'downloading %s ...\n' "$archive"
curl -fsSL -o "$tmp/$archive" "$archive_url" || die "archive download failed"

printf 'verifying checksum ...\n'
curl -fsSL -o "$tmp/checksums.txt" "$checksums_url" || die "checksum download failed"
(
  cd "$tmp"
  grep -F "  $archive" checksums.txt | shasum -a 256 -c - >/dev/null
) || die "checksum mismatch"

tar -xzf "$tmp/$archive" -C "$tmp"
install -m 0755 "$tmp/$BIN" "$install_dir/$BIN"

cat <<MSG

macontrol $tag installed to $install_dir/$BIN.

Next steps:

  $BIN setup                      # paste bot token + user ids
  $BIN service install            # auto-start at login
  $BIN doctor                     # check brew deps + sudoers

Optional brew deps for the full feature set:

  brew install brightness blueutil terminal-notifier smctemp imagesnap
MSG
