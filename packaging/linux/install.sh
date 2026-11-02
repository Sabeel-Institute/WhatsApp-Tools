#!/bin/sh
# Installs Sabeel WhatsApp Tools for the current user (no administrator
# password needed). Run it from the folder you unpacked:
#
#     ./install.sh
#
# It copies the app to ~/.local/bin and adds it to your apps menu.
set -eu

here=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
name=sabeel-whatsapp-tools
bin_dir="$HOME/.local/bin"
app_dir="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
icon_root="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor"

if [ ! -f "$here/$name" ]; then
	echo "Can't find $name next to this script. Please run it from the unpacked folder." >&2
	exit 1
fi

mkdir -p "$bin_dir" "$app_dir"
install -m 0755 "$here/$name" "$bin_dir/$name"

for size in 16 24 32 48 64 128 256 512; do
	if [ -f "$here/icons/icon-$size.png" ]; then
		mkdir -p "$icon_root/${size}x${size}/apps"
		install -m 0644 "$here/icons/icon-$size.png" "$icon_root/${size}x${size}/apps/$name.png"
	fi
done

sed "s|@BIN@|$bin_dir/$name|g" "$here/$name.desktop" >"$app_dir/$name.desktop"
chmod 0644 "$app_dir/$name.desktop"

if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database "$app_dir" >/dev/null 2>&1 || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
	gtk-update-icon-cache -q -t "$icon_root" >/dev/null 2>&1 || true
fi

echo "Installed. Open \"Sabeel WhatsApp Tools\" from your apps menu."
echo "(To try it safely first, right-click it and choose \"Try it in demo mode\".)"
