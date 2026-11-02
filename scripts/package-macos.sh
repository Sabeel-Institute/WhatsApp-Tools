#!/bin/sh
# Assembles a macOS .app bundle around an already-built binary.
#
#   scripts/package-macos.sh <binary> <version> <output dir>
#
# Produces "<output dir>/Sabeel WhatsApp Tools.app". On a Mac, the bundle is
# then ad-hoc signed; on other systems it is left unsigned (the Go linker
# already ad-hoc signs arm64 binaries).
#
# Code signing and notarization with a Developer ID are out of scope; when
# a certificate is available, replace the ad-hoc step with:
#   codesign --force --deep --options runtime --timestamp \
#     --sign "Developer ID Application: <Name> (<TEAMID>)" "$app"
#   xcrun notarytool submit <zip> --keychain-profile <profile> --wait
#   xcrun stapler staple "$app"
set -eu

if [ $# -ne 3 ]; then
	echo "usage: $0 <binary> <version> <output dir>" >&2
	exit 2
fi
binary=$1
version=$2
out=$3
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)

if [ ! -f "$binary" ]; then
	echo "error: binary $binary not found" >&2
	exit 1
fi
case "$version" in
'' | *[!0-9A-Za-z.+-]*)
	echo "error: version '$version' has unexpected characters" >&2
	exit 1
	;;
esac

app="$out/Sabeel WhatsApp Tools.app"
rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp "$binary" "$app/Contents/MacOS/sabeel-whatsapp-tools"
chmod 0755 "$app/Contents/MacOS/sabeel-whatsapp-tools"
cp "$root/assets/icon/icon.icns" "$app/Contents/Resources/AppIcon.icns"
# CFBundleVersion must be numeric dots only.
plist_version=$(printf '%s' "$version" | sed 's/^v//; s/[^0-9.].*$//')
[ -n "$plist_version" ] || plist_version=0.0.0
sed "s/@VERSION@/$plist_version/g" "$root/packaging/macos/Info.plist.in" >"$app/Contents/Info.plist"
printf 'APPL????' >"$app/Contents/PkgInfo"

if command -v codesign >/dev/null 2>&1; then
	codesign --force --deep --sign - "$app"
	codesign --verify --deep --strict "$app"
fi
echo "built $app"
