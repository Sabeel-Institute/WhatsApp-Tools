#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = ["cairosvg", "pillow", "numpy"]
# ///
"""
Build the app's icons and in-app logo from the Sabeel brand repository.

    uv run scripts/make-icons.py --brand ../brand

The icon follows the Sabeel app family (the brand's skills/sabeel-brand/tools/
make-app-icons.py): the earlier calligraphy mark on ivory with a raspberry
badge naming the app. This app's badge is a group of people. The brand tool
draws the composition; this script only adds the glyph and the desktop
formats (.png sizes, .ico, .icns) and copies the logo for the UI.

Outputs (committed, so builds don't need Python):
  assets/icon/icon-<size>.png   16..1024
  assets/icon/icon.ico          Windows
  assets/icon/icon.icns         macOS
  web/public/favicon.png        browser tab
  web/public/logo.png           the main logo, sized for the UI
"""

from __future__ import annotations

import argparse
import importlib.util
import sys
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parent.parent
SIZES = [16, 24, 32, 48, 64, 128, 256, 512, 1024]


def load_brand_tool(brand: Path):
    tool = brand / "skills" / "sabeel-brand" / "tools" / "make-app-icons.py"
    if not tool.is_file():
        sys.exit(f"error: {tool} not found; pass --brand <path to the Sabeel-Institute/brand repository>")
    spec = importlib.util.spec_from_file_location("sabeel_icons", tool)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def group_glyph(on: str, plum: str, cx: float, cy: float, r: float) -> str:
    """Two people, drawn in the brand tool's -100..100 badge space."""
    return (
        f'<g transform="translate({cx},{cy}) scale({r / 100.0})">'
        f'<g fill="{on}">'
        # the person behind
        '<circle cx="24" cy="-30" r="17"/>'
        '<path d="M-4 30 C-2 -2 50 -2 54 30 Z"/>'
        "</g>"
        # the person in front, outlined in plum so the two stay distinct
        f'<g fill="{on}" stroke="{plum}" stroke-width="10" stroke-linejoin="round">'
        '<circle cx="-14" cy="-16" r="22"/>'
        '<path d="M-58 46 C-56 6 28 6 30 46 Z"/>'
        "</g>"
        "</g>"
    )


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--brand", required=True, type=Path, help="path to the Sabeel-Institute/brand repository")
    args = ap.parse_args()
    brand = args.brand.resolve()
    tool = load_brand_tool(brand)

    original = tool.badge_glyph

    def glyph(kind, cx, cy, r):
        if kind == "group":
            return group_glyph(tool.ON_PLUM, tool.PLUM, cx, cy, r)
        return original(kind, cx, cy, r)

    tool.badge_glyph = glyph
    mark = Image.open(tool.EARLIER_MARK).convert("RGBA")
    max_r, hidden, square_r = tool.geometry(mark, "group")
    if hidden or square_r > tool.SQUARE_R:
        sys.exit(f"error: icon geometry check failed (hidden={hidden}, square_r={square_r:.1f})")
    print(f"geometry ok: farthest ink {square_r:.1f} of {tool.SQUARE_R}, calligraphy hidden {hidden}px")

    master = tool.render(tool.icon_svg(mark, "group", scale=tool.SQUARE_SCALE)).convert("RGBA")
    out = ROOT / "assets" / "icon"
    out.mkdir(parents=True, exist_ok=True)
    for size in SIZES:
        master.resize((size, size), Image.LANCZOS).save(out / f"icon-{size}.png", optimize=True)
    master.save(out / "icon.ico", sizes=[(s, s) for s in (16, 24, 32, 48, 64, 128, 256)])
    master.save(out / "icon.icns")

    public = ROOT / "web" / "public"
    public.mkdir(parents=True, exist_ok=True)
    master.resize((64, 64), Image.LANCZOS).save(public / "favicon.png", optimize=True)

    logo = Image.open(brand / "skills" / "sabeel-brand" / "logo" / "sabeel-logo.png").convert("RGBA")
    width = 560
    logo.resize((width, round(logo.height * width / logo.width)), Image.LANCZOS).save(public / "logo.png", optimize=True)
    earlier = Image.open(tool.EARLIER_MARK).convert("RGBA")
    width = 160
    earlier.resize((width, round(earlier.height * width / earlier.width)), Image.LANCZOS).save(
        public / "mark.png", optimize=True)
    print(f"wrote icons to {out} and UI images to {public}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
