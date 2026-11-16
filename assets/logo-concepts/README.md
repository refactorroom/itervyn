# Itervyn logo concepts

Three manually constructed vector reinterpretations of `../itervyn-panda-icon.svg`, informed by the project README. Original assets are untouched. Local concept creation only; no publication or identity replacement.

## Directions

| Concept | Meaning and construction | Typography and color | Best use | Tradeoff |
| --- | --- | --- | --- | --- |
| Terminal Panda — recommended | Broad panda silhouette with the original terminal visor reduced to `>_`. Ears and visor form its memorable features. | Bold neutral Itervyn wordmark; ink, mint and porcelain retain the source's terminal character. | CLI identity, README, avatar | Prompt becomes less distinct at 16 px; use 24 px or larger when that detail matters. |
| Iteration Loop | Two arrows surround a panda: execute, verify, refine, repeat. | Same wordmark; mint gives the repeat cycle emphasis. | Process-oriented branding and larger illustrations | More familiar cycle metaphor; arrows require more space than the first concept. |
| Bracket Panda | Code brackets surround a panda whose eyes echo angle brackets. | Same wordmark; mint separates the coding frame from the mascot. | Developer docs and repository branding | Wider construction and less close to the original visor identity. |

## Files

- `*-icon.svg`: editable vector symbols, 256 × 256 canvas, transparent.
- `*-icon.png`: 512 × 512 transparent raster symbols.
- `*-logo.svg`: horizontal wordmarks, 520 × 192 canvas, transparent.
- `*-logo.png`: horizontal wordmarks, 1040 × 384, transparent.
- `terminal-panda-dark.*`, `terminal-panda-mono.*`, `terminal-panda-mono-dark.*`: recommended symbol variants; SVG 256 × 256, PNG 512 × 512. Dark variants are designed for #101B29 backgrounds.
- `concept-preview.*`: comparison board, 1600 × 1000.
- `size-and-contrast-check.*`: recommended concept at native 16, 24, 32 and 64 px on light/dark backgrounds, color and monochrome; 1040 × 580.

The primary logo is for light backgrounds. Use the dark symbol on dark backgrounds. Preserve proportions and at least one ear radius of surrounding space.

Palette: ink #121D2B, mint #27DCC1, porcelain #F4F7F6. These are exploratory designs, not a user-approved final identity.

## Provenance and portability

Manually authored SVG geometry based on the user-supplied source. No generative image model or external artwork used. Original artwork rights are assumed from its supplied project context; not independently established. Wordmarks use live Arial Bold, with Helvetica and sans-serif fallbacks. No font files are distributed. SVG wordmark metrics can change when Arial is unavailable; PNGs preserve the inspected appearance. Wordmarks are editable text rather than outlined glyphs.

## Verification

- **Passed:** SVG XML parsing and rendering with librsvg; sources contain no scripts, external references, embedded raster artwork or remote fonts.
- **Passed:** visual inspection of all three concepts on the comparison board; exact spelling and casing `Itervyn`.
- **Passed:** separate horizontal Terminal Panda lockup inspected; no clipping observed.
- **Passed:** recommended symbol visually checked at native 24 and 32 px in color and monochrome, on white and #101B29. Monochrome prompt and dark-background ears were corrected after the initial render.
- **Passed with limitation:** native 16 px silhouette checked; terminal prompt loses clarity. These are concept symbols rather than a separately optimized favicon package.
- **Passed:** PNG format, dimensions and alpha-channel metadata checked. Transparent logo and icon outputs rendered successfully; preview boards intentionally have opaque backgrounds.
- **Not Run:** individual small-scale light/dark and monochrome checks for the two alternate concepts; supplied as exploratory options.
- **Not Run:** cross-platform font comparison and outlined-type export.
- **Not Run:** trademark clearance or marketplace uniqueness research.
