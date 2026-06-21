package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/spf13/cobra"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

var (
	inputPath  string
	outputPath string
	pwa        bool
	color      string
)

var faviconSizes = []struct{ W, H int }{
	{57, 57}, {60, 60}, {72, 72}, {76, 76},
	{114, 114}, {120, 120}, {144, 144}, {152, 152},
	{180, 180}, {16, 16}, {32, 32}, {96, 96},
	{192, 192}, {70, 70}, {150, 150}, {310, 310},
}

var pwaFaviconSizes = []struct{ W, H int }{
	{128, 128}, {384, 384}, {512, 512},
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "favycon [flags] <input-image>",
		Short: "Generate favicon files from an input image",
		Long:  "Favycon is a CLI tool that generates all the favicon files you need from a single PNG, JPEG, or SVG image.",
		Args:  cobra.ExactArgs(1),
		RunE:  run,
	}

	rootCmd.Flags().StringVarP(&outputPath, "output", "o", "favicons.zip", "Output zip file path")
	rootCmd.Flags().BoolVarP(&pwa, "pwa", "p", false, "Include PWA manifest and icons")
	rootCmd.Flags().StringVarP(&color, "color", "c", "#ffffff", "Theme color for browserconfig.xml and manifest.json")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	inputPath = args[0]

	isSvg := strings.EqualFold(filepath.Ext(inputPath), ".svg")

	var src image.Image
	var err error

	if isSvg {
		src, err = rasterizeSvg(inputPath, 512)
		if err != nil {
			return fmt.Errorf("failed to rasterize SVG: %w", err)
		}
	} else {
		src, err = imaging.Open(inputPath)
		if err != nil {
			return fmt.Errorf("failed to open input image: %w", err)
		}
	}

	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	minSize := 310
	if pwa {
		minSize = 512
	}

	if !isSvg && (w < minSize || h < minSize) {
		return fmt.Errorf("input image should be at least %dpx, got %dx%d", minSize, w, h)
	}

	sizes := make([]struct{ W, H int }, len(faviconSizes))
	copy(sizes, faviconSizes)
	if pwa {
		sizes = append(sizes, pwaFaviconSizes...)
	}

	var buf bytes.Buffer
	zipw := zip.NewWriter(&buf)

	for _, s := range sizes {
		resized := imaging.Resize(src, s.W, s.H, imaging.Lanczos)
		var entry bytes.Buffer
		if err := png.Encode(&entry, resized); err != nil {
			return fmt.Errorf("failed to encode PNG %dx%d: %w", s.W, s.H, err)
		}
		name := fmt.Sprintf("icons/favicon-%dx%d.png", s.W, s.H)
		if err := addToZip(zipw, name, entry.Bytes()); err != nil {
			return err
		}
	}

	ico := generateIco(src)
	if err := addToZip(zipw, "icons/favicon.ico", ico); err != nil {
		return err
	}

	if isSvg {
		data, err := os.ReadFile(inputPath)
		if err != nil {
			return fmt.Errorf("failed to read SVG file: %w", err)
		}
		if err := addToZip(zipw, "icons/favicon.svg", data); err != nil {
			return err
		}
	}

	if pwa {
		manifest := generateManifest(color)
		if err := addToZip(zipw, "icons/manifest.json", []byte(manifest)); err != nil {
			return err
		}
	}

	bc := generateBrowserConfig(color)
	if err := addToZip(zipw, "icons/browserconfig.xml", []byte(bc)); err != nil {
		return err
	}

	readme := generateReadme(color, isSvg, pwa)
	if err := addToZip(zipw, "readme.txt", []byte(readme)); err != nil {
		return err
	}

	if err := zipw.Close(); err != nil {
		return fmt.Errorf("failed to close zip: %w", err)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Generated %s with %d icon sizes\n", outputPath, len(sizes)+1)
	return nil
}

func rasterizeSvg(path string, size int) (image.Image, error) {
	if p, err := exec.LookPath("resvg"); err == nil {
		tmp, err := os.CreateTemp("", "favycon-*.png")
		if err != nil {
			return nil, err
		}
		tmpName := tmp.Name()
		tmp.Close()
		defer os.Remove(tmpName)

		cmd := exec.Command(p, "-w", fmt.Sprintf("%d", size), "-h", fmt.Sprintf("%d", size), path, tmpName)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("resvg failed: %s: %w", out, err)
		}
		return imaging.Open(tmpName)
	}

	icon, err := oksvg.ReadIcon(path, oksvg.StrictErrorMode)
	if err != nil {
		return nil, err
	}
	w, h := int(math.Ceil(icon.ViewBox.W)), int(math.Ceil(icon.ViewBox.H))
	if w == 0 || h == 0 {
		w, h = size, size
	}
	scale := float64(size) / math.Max(float64(w), float64(h))
	tw, th := int(math.Ceil(float64(w)*scale)), int(math.Ceil(float64(h)*scale))
	icon.SetTarget(0, 0, float64(tw), float64(th))
	img := image.NewNRGBA(image.Rect(0, 0, tw, th))
	scanner := rasterx.NewScannerGV(tw, th, img, img.Bounds())
	dasher := rasterx.NewDasher(tw, th, scanner)
	icon.Draw(dasher, 1.0)
	return img, nil
}

func addToZip(zipw *zip.Writer, name string, data []byte) error {
	w, err := zipw.Create(name)
	if err != nil {
		return fmt.Errorf("failed to create zip entry %s: %w", name, err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("failed to write zip entry %s: %w", name, err)
	}
	return nil
}

func generateIco(src image.Image) []byte {
	resized := imaging.Resize(src, 256, 256, imaging.Lanczos)
	var buf bytes.Buffer
	png.Encode(&buf, resized)

	pngData := buf.Bytes()

	var ico bytes.Buffer
	ico.Write([]byte{0, 0, 1, 0})
	ico.Write([]byte{1, 0})
	ico.Write([]byte{0, 0, 0, 0})
	ico.Write([]byte{16, 0, 0, 0})

	headerSize := 6 + 16
	dataOffset := uint32(headerSize)
	ico.Write([]byte{byte(dataOffset), byte(dataOffset >> 8), byte(dataOffset >> 16), byte(dataOffset >> 24)})
	ico.Write([]byte{byte(len(pngData)), byte(len(pngData) >> 8), byte(len(pngData) >> 16), byte(len(pngData) >> 24)})
	ico.Write([]byte{0, 0, 0, 0})
	ico.Write(pngData)

	return ico.Bytes()
}

func generateBrowserConfig(color string) string {
	type Msapplication struct {
		XMLName xml.Name `xml:"msapplication"`
		Tile    struct {
			XMLName          xml.Name `xml:"tile"`
			Square70x70Logo  struct {
				XMLName xml.Name `xml:"square70x70logo"`
				Src     string   `xml:"src,attr"`
			} `xml:"square70x70logo"`
			Square150x150Logo struct {
				XMLName xml.Name `xml:"square150x150logo"`
				Src     string   `xml:"src,attr"`
			} `xml:"square150x150logo"`
			Square310x310Logo struct {
				XMLName xml.Name `xml:"square310x310logo"`
				Src     string   `xml:"src,attr"`
			} `xml:"square310x310logo"`
			TileColor string `xml:"TileColor"`
		} `xml:"tile"`
	}

	type Browserconfig struct {
		XMLName       xml.Name       `xml:"browserconfig"`
		Msapplication Msapplication  `xml:"msapplication"`
	}

	bc := Browserconfig{}
	bc.Msapplication.Tile.Square70x70Logo.Src = "/favicon-70x70.png"
	bc.Msapplication.Tile.Square150x150Logo.Src = "/favicon-150x150.png"
	bc.Msapplication.Tile.Square310x310Logo.Src = "/favicon-310x310.png"
	bc.Msapplication.Tile.TileColor = color

	output, err := xml.MarshalIndent(bc, "", "  ")
	if err != nil {
		return ""
	}

	return xml.Header + string(output) + "\n"
}

func generateManifest(color string) string {
	return fmt.Sprintf(`{
  "name": "My App",
  "description": "My App description",
  "short_name": "My App",
  "icons": [
    { "src": "/favicon-72x72.png", "type": "image/png", "sizes": "72x72", "purpose": "any maskable" },
    { "src": "/favicon-96x96.png", "type": "image/png", "sizes": "96x96", "purpose": "any maskable" },
    { "src": "/favicon-128x128.png", "type": "image/png", "sizes": "128x128", "purpose": "any maskable" },
    { "src": "/favicon-144x144.png", "type": "image/png", "sizes": "144x144", "purpose": "any maskable" },
    { "src": "/favicon-152x152.png", "type": "image/png", "sizes": "152x152", "purpose": "any maskable" },
    { "src": "/favicon-192x192.png", "type": "image/png", "sizes": "192x192", "purpose": "any maskable" },
    { "src": "/favicon-384x384.png", "type": "image/png", "sizes": "384x384", "purpose": "any maskable" },
    { "src": "/favicon-512x512.png", "type": "image/png", "sizes": "512x512", "purpose": "any maskable" }
  ],
  "scope": "/",
  "start_url": "/?source=pwa",
  "display": "standalone",
  "theme_color": "%s",
  "background_color": "%s"
}`, color, color)
}

func generateReadme(color string, isSvg, isPwa bool) string {
	head := generateHeadTags(color, isSvg, isPwa)
	return fmt.Sprintf(`Thank you for using Favycon!

Now that you have all favicon files generated, you can copy all files
from the icons folder into your project public folder.

Last but not least add the following HTML tags to your index.html file.

%s`, head)
}

func generateHeadTags(color string, isSvg, isPwa bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<link rel="apple-touch-icon" sizes="57x57" href="/favicon-57x57.png">
<link rel="apple-touch-icon" sizes="60x60" href="/favicon-60x60.png">
<link rel="apple-touch-icon" sizes="72x72" href="/favicon-72x72.png">
<link rel="apple-touch-icon" sizes="76x76" href="/favicon-76x76.png">
<link rel="apple-touch-icon" sizes="114x114" href="/favicon-114x114.png">
<link rel="apple-touch-icon" sizes="120x120" href="/favicon-120x120.png">
<link rel="apple-touch-icon" sizes="144x144" href="/favicon-144x144.png">
<link rel="apple-touch-icon" sizes="152x152" href="/favicon-152x152.png">
<link rel="apple-touch-icon" sizes="180x180" href="/favicon-180x180.png">`)
	if isSvg {
		b.WriteString("\n<link rel=\"icon\" type=\"image/svg+xml\" href=\"/favicon.svg\">")
	}
	fmt.Fprintf(&b, `
<link rel="icon" type="image/png" sizes="16x16" href="/favicon-16x16.png">
<link rel="icon" type="image/png" sizes="32x32" href="/favicon-32x32.png">
<link rel="icon" type="image/png" sizes="96x96" href="/favicon-96x96.png">
<link rel="icon" type="image/png" sizes="192x192" href="/favicon-192x192.png">
<link rel="shortcut icon" type="image/x-icon" href="/favicon.ico">
<link rel="icon" type="image/x-icon" href="/favicon.ico">
<meta name="msapplication-TileColor" content="%s">
<meta name="msapplication-TileImage" content="/favicon-144x144.png">
<meta name="msapplication-config" content="/browserconfig.xml">`, color)
	if isPwa {
		fmt.Fprintf(&b, "\n<link rel=\"manifest\" href=\"/manifest.json\">\n<meta name=\"theme-color\" content=\"%s\">", color)
	}
	return b.String()
}
