Font Awesome Free 6.7.2 by Fonticons, Inc.

Source: https://github.com/FortAwesome/Font-Awesome/tree/6.7.2/svgs

These SVG icons are licensed under CC BY 4.0. Attribution comments
are preserved; see LICENSE.txt and https://fontawesome.com/license/free.
Only the icons used by the application and its Fyne dialogs are included.

SVG paths containing arcs are converted to cubic curves to work around Fyne's
renderer mishandling repeated arc segments. Other paths are kept. Checkbox canvases are cropped to their square artwork
bounds so Fyne does not stretch the ticks into rectangles.
Run `python3 scripts/normalize-icons.py` from the repository after updating SVGs
(requires fonttools 4.64.0). This is not needed to build or run the application.
