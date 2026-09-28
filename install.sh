#!/bin/sh
# Install fotoly and pixelfox from GitHub releases.
# macOS and Linux:
#   curl -fsSL https://raw.githubusercontent.com/ManuelReschke/fotoly-cli/main/install.sh | sh
# Pin a version or directory:
#   curl -fsSL https://raw.githubusercontent.com/ManuelReschke/fotoly-cli/main/install.sh | sh -s -- --version 1.0.0
#   FOTOLY_INSTALL_DIR=$HOME/bin sh install.sh
set -eu

repo="ManuelReschke/fotoly-cli"
version="${FOTOLY_VERSION:-}"
dest="${FOTOLY_INSTALL_DIR:-}"

usage() {
	cat <<'EOF'
Usage: install.sh [--version <tag>] [--prefix <dir>]

Downloads the fotoly and pixelfox binaries for this machine and installs both.
The default directory is ~/.local/bin, or /usr/local/bin when run as root.

  --version   release tag, with or without a leading v (default: latest)
  --prefix    install directory (overrides FOTOLY_INSTALL_DIR)
EOF
}

die() {
	echo "install.sh: $*" >&2
	exit 1
}

while [ $# -gt 0 ]; do
	case "$1" in
	--version)
		[ $# -ge 2 ] || die "--version needs a value"
		version=$2
		shift 2
		;;
	--version=*)
		version=${1#--version=}
		shift
		;;
	--prefix)
		[ $# -ge 2 ] || die "--prefix needs a value"
		dest=$2
		shift 2
		;;
	--prefix=*)
		dest=${1#--prefix=}
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		die "unknown argument: $1"
		;;
	esac
done

if ! command -v curl >/dev/null 2>&1; then
	die "curl is required"
fi

os=$(uname -s)
case "$os" in
Linux) os=linux ;;
Darwin) os=darwin ;;
*) die "unsupported operating system: $os" ;;
esac

arch=$(uname -m)
case "$arch" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) die "unsupported architecture: $arch" ;;
esac

if [ -z "$dest" ]; then
	if [ "$(id -u)" -eq 0 ]; then
		dest=/usr/local/bin
	else
		[ -n "${HOME:-}" ] || die "HOME is not set; pass --prefix"
		dest=$HOME/.local/bin
	fi
fi

if [ -z "$version" ]; then
	latest=$(curl -fsSL -A fotoly-cli-installer -o /dev/null -w '%{url_effective}' "https://github.com/${repo}/releases/latest") || die "could not resolve the latest release"
	version=${latest##*/}
fi
version=${version#v}
case "$version" in
"" | */*) die "invalid version: ${version}" ;;
esac
tag=v$version
asset=fotoly-cli_${version}_${os}_${arch}.tar.gz
base="https://github.com/${repo}/releases/download/${tag}"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

echo "Installing fotoly and pixelfox ${tag} for ${os}/${arch}"
curl -fsSL -A fotoly-cli-installer -o "$tmp/SHA256SUMS" "${base}/SHA256SUMS" || die "could not download checksums for ${tag}"
curl -fsSL -A fotoly-cli-installer -o "$tmp/$asset" "${base}/${asset}" || die "could not download ${asset}"

expected=$(awk -v name="$asset" '$2 == name { print $1; exit }' "$tmp/SHA256SUMS")
[ -n "$expected" ] || die "no checksum published for ${asset}"
if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$tmp/$asset" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
	actual=$(shasum -a 256 "$tmp/$asset" | awk '{ print $1 }')
else
	die "sha256sum or shasum is required"
fi
[ "$expected" = "$actual" ] || die "checksum mismatch for ${asset}"

mkdir -p "$tmp/extract"
tar -xzf "$tmp/$asset" -C "$tmp/extract"
stem=fotoly-cli_${version}_${os}_${arch}
[ -f "$tmp/extract/$stem/fotoly" ] && [ -f "$tmp/extract/$stem/pixelfox" ] || die "archive is missing fotoly or pixelfox"

mkdir -p "$dest"
if command -v install >/dev/null 2>&1; then
	install -m 755 "$tmp/extract/$stem/fotoly" "$dest/fotoly"
	install -m 755 "$tmp/extract/$stem/pixelfox" "$dest/pixelfox"
else
	cp "$tmp/extract/$stem/fotoly" "$dest/fotoly"
	cp "$tmp/extract/$stem/pixelfox" "$dest/pixelfox"
	chmod 755 "$dest/fotoly" "$dest/pixelfox"
fi

echo "Installed ${tag} to ${dest}"
echo "  ${dest}/fotoly"
echo "  ${dest}/pixelfox"
case ":$PATH:" in
*":$dest:"*) ;;
*)
	echo
	echo "Add ${dest} to PATH:"
	echo "  export PATH=\"${dest}:\$PATH\""
	;;
esac
echo
echo "Next: fotoly setup"
