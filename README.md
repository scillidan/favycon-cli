# favycon-cli

CLI port of [Favycon](https://github.com/ruisaraiva19/favycon), a web-based favicon generator. This tool reimplements the same icon generation logic as a standalone CLI. To generate all favicon files from a single PNG, JPEG, or SVG image.

Authors: GLM-5.1🧙‍♂️, scillidan🤡

## Install

```bash
go install github.com/scillidan/favycon-cli@latest
```

Or download a binary from [Releases](https://github.com/scillidan/favycon-cli/releases).

## Usage

```bash
favycon [flags] <input-image>
```

|Flag      |Short|Default       |Description                                        |
|:-        |:-   |:-            |:-                                                 |
|`--output`|`-o` |`favicons.zip`|Output zip file path                               |
|`--pwa`   |`-p` |`false`       |Include PWA manifest and 128/384/512 icons         |
|`--color` |`-c` |`#ffffff`     |Theme color for browserconfig.xml and manifest.json|

### Examples

```bash
favycon icon.png
favycon icon.png -o assets/favicons.zip
favycon icon.png --pwa -c "#000000"
```

The input image must be at least 310px (512px with `--pwa`).

### Output

The generated zip contains:

- `icons/favicon-{size}.png` — 16 sizes (57–310px), plus 3 PWA sizes with `--pwa`
- `icons/favicon.ico` — ICO from 256×256 PNG
- `icons/favicon.svg` — copied from input (SVG only)
- `icons/manifest.json` — PWA web app manifest (`--pwa` only)
- `icons/browserconfig.xml` — IE10+ tile config
- `readme.txt` — HTML head tags to paste into your page

## Build from Source

```bash
make build        # build for current platform
make dist         # cross-compile all platforms to dist/
```