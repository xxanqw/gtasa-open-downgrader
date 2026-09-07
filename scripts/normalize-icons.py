#!/usr/bin/env python3
"""Normalize vendored Font Awesome SVG arcs for Fyne's renderer.

Maintenance only: python3 -m pip install fonttools==4.64.0
Run after updating icons; normal Go builds use the committed SVGs directly.
"""
from pathlib import Path
import re

from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.svgLib.path import parse_path

root = Path(__file__).resolve().parents[1] / "internal/assets/fontawesome"
notice = "<!-- SVG arc paths converted to cubic curves for Fyne compatibility. -->"
for path in sorted(root.glob("*.svg")):
    source = path.read_text()

    def normalize(match):
        data = match.group(1)
        if not re.search("[aA]", data):
            return match.group(0)
        pen = SVGPathPen(None, ntos=lambda value: format(value, ".6f").rstrip("0").rstrip("."))
        parse_path(data, pen)
        return 'd="' + pen.getCommands() + '"'

    normalized = re.sub(r'\bd="([^"]+)"', normalize, source)
    if normalized != source:
        normalized = normalized.replace("</svg>", notice + "</svg>")
        path.write_text(normalized)
        print(path.name)
