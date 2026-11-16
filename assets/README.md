# Itervyn brand kit: Cyborg Panda

This kit uses **`itervyn-panda-icon.svg` as the master**, retaining its original panda, visor, circular framing, IVN badge, gradients, teal and pink accents. The folder organization follows `../../invaris/assets/`.

## Contents

- `logos/`: Itervyn horizontal lockups on dark, light and transparent backgrounds; full-color symbols with and without the source backdrop. Transparent horizontal wordmark is light-colored for dark surfaces.
- `icons/`: 1024 px avatar, 512 px app icon, favicon SVG and PNGs at 16, 32, 48, 64, 128 and 256 px.
- `banners/`: README banner (1600 × 480), social preview (1280 × 640), docs cover (1600 × 700), launch poster (1600 × 900), terminal illustration (1600 × 600), each SVG and PNG.
- `social/`: feature card (1200 × 630) and square post (1080 × 1080), SVG and PNG.
- `original/`: byte-for-byte copy of the supplied SVG master.
- `preview/`: contact sheet and native-size checks.
- `palette.json`: colors sampled from the supplied identity.
- `manifest.json`: generated asset paths and dimensions; original-master SHA-256.

The original root SVG files remain untouched. `logo-concepts/` contains earlier exploratory redesigns and is **not part of this kit**. No site, README integration, Git operations or external publication was performed.

## Use

README snippet (not yet added to the repository README):

```md
![Itervyn](assets/banners/github-readme-1600x480.png)
```

Use `banners/github-social-preview-1280x640.png` as an upload asset for a repository social preview. The avatar file can be used in a supported profile context; a repository does not have its own independently uploaded avatar. No platform settings were changed or platform-specific compliance certified.

Unlike the Invaris kit, monochrome recolorings are not included: this package preserves your chosen full-color artwork. The original rounded background is retained in icons and most lockups. The transparent symbol variant removes only that background rectangle. Transparent areas include the rounded corners on the original tile and the surrounding canvas on composed logos.

## Provenance and typography

Artwork supplied by the user. New compositions are manually assembled vectors; no generative image model or new mascot was used. Artwork ownership is assumed from the supplied project context, not independently verified. `Itervyn` wordmarks are outlined vector paths generated locally from installed Arial Bold via CoreText; no font files are distributed. Other captions use live Arial with Helvetica/sans-serif fallbacks. The original IVN badge retains its source font stack (Inter Display, Inter, sans-serif), so badge lettering may vary between SVG renderers. PNGs preserve the rendered appearance.

The terminal illustration shows documented command forms and no synthetic verification results. It is a brand illustration, not a CLI screenshot.

## Verification

- **Passed:** original-master SHA-256 matches the copied reference; original bytes unchanged.
- **Passed:** every generated SVG parses and renders with librsvg; no scripts, external references, or raster artwork embedded in SVG.
- **Passed:** generated PNG signatures and dimensions match the manifest; transparent logo exports contain an alpha channel.
- **Passed:** contact sheet visually inspected for composition, clipping, spelling and master-artwork fidelity.
- **Passed with limitation:** original symbol inspected at native 16, 24 and 32 px on light/dark surfaces. Detailed rings, visor glyphs and badge are not reliably legible at 16–24 px; use larger sizes for the full identity. Favicon exports faithfully scale the original, without redesigning it.
- **Not Run:** monochrome testing (no monochrome variants), cross-platform font comparison, trademark clearance and marketplace uniqueness research.
