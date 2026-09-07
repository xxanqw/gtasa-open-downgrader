#!/usr/bin/env python3
"""Normalize vendored Font Awesome SVG arcs for Fyne's renderer.

Maintenance only: python3 -m pip install fonttools==4.64.0
Run after updating icons; normal Go builds use the committed SVGs directly.
"""
from pathlib import Path
import re

from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
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
    if path.name in {"regular-square.svg", "solid-square.svg", "solid-square-check.svg", "solid-square-minus.svg"}:
        # The artwork is square, but the upstream canvas has vertical padding.
        # Fyne stretches checkbox images into square controls, distorting it.
        if 'viewBox="0 0 448 512"' in normalized or 'viewBox="0 32 448 448"' in normalized:
            def square_canvas(match):
                pen = SVGPathPen(None, ntos=lambda value: format(value, ".6f").rstrip("0").rstrip("."))
                parse_path(match.group(1), TransformPen(pen, (1, 0, 0, 1, 0, -32)))
                return 'd="' + pen.getCommands() + '"'
            normalized = re.sub(r'\bd="([^"]+)"', square_canvas, normalized)
            normalized = re.sub(r'viewBox="0 (?:0 448 512|32 448 448)"', 'viewBox="0 0 448 448"', normalized)
        if normalized != source and "Checkbox canvas" not in normalized:
            normalized = normalized.replace("</svg>", "<!-- Checkbox canvas cropped to square artwork bounds. --></svg>")
    if normalized != source:
        path.write_text(normalized)
        print(path.name)
