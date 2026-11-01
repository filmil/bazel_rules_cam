# Toolpath of //cam/private/testing:plate_linuxcnc

Stock: X -2.000 to 42.000, Y -2.000 to 32.000, Z 0.000 to 11.000 mm (top at Z 11.000).

| Operation | Tools | Levels (Z, mm) | Deepest Z | Cut (mm) | Rapid (mm) | Time (min) |
|---|---|---|---|---|---|---|
| Pocket | T1 (3 mm endmill) | 8, 6 | 6.000 | 258.0 | 78.7 | 0.6 |
| Profile | T1 (3 mm endmill) | 8, 5, 2, 0 | 0.000 | 611.7 | 41.2 | 1.1 |

Estimated time: 1.7 min (feeds as written; rapids at 5000 mm/min; no acceleration).

In the pictures, cuts are colored by depth: light blue at the stock's top, through blue and purple, to dark red at the deepest cut. Rapids are dashed gray, and the stock is the gray box.
