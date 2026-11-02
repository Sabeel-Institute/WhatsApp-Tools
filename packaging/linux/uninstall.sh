#!/bin/sh
# Removes Sabeel WhatsApp Tools from this user account. Your WhatsApp link,
# settings and history stay in ~/.config/sabeel-whatsapp-tools unless you
# pass --remove-data.
set -eu

name=sabeel-whatsapp-tools
data_root="${XDG_DATA_HOME:-$HOME/.local/share}"
config_dir="${XDG_CONFIG_HOME:-$HOME/.config}/$name"

remove_data=false
for arg in "$@"; do
	case "$arg" in
	--remove-data) remove_data=true ;;
	*)
		echo "Unknown option: $arg (the only option is --remove-data)" >&2
		exit 2
		;;
	esac
done

rm -f "$HOME/.local/bin/$name" "$data_root/applications/$name.desktop"
for dir in "$data_root"/icons/hicolor/*/apps; do
	rm -f "$dir/$name.png"
done

if [ "$remove_data" = true ]; then
	rm -rf "$config_dir"
	echo "Removed the app and its data."
else
	echo "Removed the app. Your data is still in $config_dir"
	echo "(run with --remove-data to delete it too)."
fi
