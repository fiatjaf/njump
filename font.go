package main

import (
	"embed"
	"fmt"
	"image"
	"image/draw"
	"io/fs"
	"path/filepath"

	"golang.org/x/exp/slices"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

//go:embed fonts/*
var fontDir embed.FS

// load all fonts
var fonts, fontFiles = func() ([]*sfnt.Font, []fs.DirEntry) {
	res := make([]*sfnt.Font, 0, 20)
	dir, err := fontDir.ReadDir("fonts")
	if err != nil {
		panic(fmt.Errorf("error reading embedded fonts dir: %w", err))
	}

	slices.SortFunc(dir, func(a, b fs.DirEntry) int {
		if a.Name() == "NotoSans.ttf" {
			return -1
		}
		return 0
	})

	for _, entry := range dir {
		fontData, _ := fontDir.ReadFile(filepath.Join("fonts", entry.Name()))
		f, _ := sfnt.Parse(fontData)
		res = append(res, f)
	}

	return res, dir
}()

func renderText(dst draw.Image, lines []string) {
	var b sfnt.Buffer
	rect := dst.Bounds()
	for l, line := range lines {
		for c, glyph := range line {
			fmt.Printf("rendering %s\n", string(glyph))
			r := vector.NewRasterizer(300, 600)
			if err := renderGlyph(r, glyph, FONT_SIZE, b); err != nil {
				fmt.Println("error:", err)
				continue
			}
			rect.Min.Y = 10 + (l*FONT_SIZE*FONT_DPI*256.0/72.0)>>8
			rect.Min.X = FONT_SIZE * c
			fmt.Println("rect", rect)
			r.Draw(dst, rect, image.Opaque, image.Point{})
		}
	}
}

func renderGlyph(r *vector.Rasterizer, glyph rune, ppem int, b sfnt.Buffer) error {
	font, index := getFontForGlyph(glyph, fonts, b)
	if font == nil {
		return fmt.Errorf("can't render this '%v' character with any font", glyph)
	}

	segments, err := font.LoadGlyph(&b, index, fixed.I(ppem), nil)
	if err != nil {
		return fmt.Errorf("error loading glyph '%v' from index %v at font %v: %w", glyph, index, font, err)
	}

	var originX float32 = 10
	var originY float32 = 25

	// translate and scale that glyph as we pass it to a vector.Rasterizer.
	r.DrawOp = draw.Src
	for _, seg := range segments {
		// the divisions by 64 below is because the seg.Args values have type
		// fixed.Int26_6, a 26.6 fixed point number, and 1<<6 == 64.
		switch seg.Op {
		case sfnt.SegmentOpMoveTo:
			r.MoveTo(
				originX+float32(seg.Args[0].X)/64,
				originY+float32(seg.Args[0].Y)/64,
			)
		case sfnt.SegmentOpLineTo:
			r.LineTo(
				originX+float32(seg.Args[0].X)/64,
				originY+float32(seg.Args[0].Y)/64,
			)
		case sfnt.SegmentOpQuadTo:
			r.QuadTo(
				originX+float32(seg.Args[0].X)/64,
				originY+float32(seg.Args[0].Y)/64,
				originX+float32(seg.Args[1].X)/64,
				originY+float32(seg.Args[1].Y)/64,
			)
		case sfnt.SegmentOpCubeTo:
			r.CubeTo(
				originX+float32(seg.Args[0].X)/64,
				originY+float32(seg.Args[0].Y)/64,
				originX+float32(seg.Args[1].X)/64,
				originY+float32(seg.Args[1].Y)/64,
				originX+float32(seg.Args[2].X)/64,
				originY+float32(seg.Args[2].Y)/64,
			)
		}
	}

	return nil
}

func getFontForGlyph(glyph rune, fonts []*sfnt.Font, b sfnt.Buffer) (*sfnt.Font, sfnt.GlyphIndex) {
	for i, font := range fonts {
		glyphIndex, err := font.GlyphIndex(&b, glyph)
		if err != nil {
			continue
		}
		if glyphIndex == 0 {
			continue
		}

		fmt.Println("picking font", fontFiles[i].Name())
		return font, glyphIndex
	}
	return nil, 0
}

/// package main
///
/// import (
/// 	"image"
/// 	"image/draw"
/// 	"image/png"
/// 	"log"
/// 	"os"
///
/// 	"golang.org/x/image/font/sfnt"
/// 	"golang.org/x/image/math/fixed"
/// 	"golang.org/x/image/vector"
/// )
///
/// func main() {
/// 	const (
/// 		ppem    = 320
/// 		width   = 500
/// 		height  = 560
/// 		originX = 0
/// 		originY = 320
/// 	)
///
/// 	emojiFont, _ := os.ReadFile("NotoEmoji.ttf")
/// 	f, err := sfnt.Parse(emojiFont)
/// 	if err != nil {
/// 		log.Fatalf("Parse: %v", err)
/// 	}
/// 	var b sfnt.Buffer
/// 	x, err := f.GlyphIndex(&b, '🍰')
/// 	if err != nil {
/// 		log.Fatalf("GlyphIndex: %v", err)
/// 	}
/// 	if x == 0 {
/// 		log.Fatalf("GlyphIndex: no glyph index found for the rune 'Ġ'")
/// 	}
/// 	segments, err := f.LoadGlyph(&b, x, fixed.I(ppem), nil)
/// 	if err != nil {
/// 		log.Fatalf("LoadGlyph: %v", err)
/// 	}
///
/// 	// Translate and scale that glyph as we pass it to a vector.Rasterizer.
/// 	r := vector.NewRasterizer(width, height)
/// 	r.DrawOp = draw.Src
/// 	for _, seg := range segments {
/// 		// The divisions by 64 below is because the seg.Args values have type
/// 		// fixed.Int26_6, a 26.6 fixed point number, and 1<<6 == 64.
/// 		switch seg.Op {
/// 		case sfnt.SegmentOpMoveTo:
/// 			r.MoveTo(
/// 				originX+float32(seg.Args[0].X)/64,
/// 				originY+float32(seg.Args[0].Y)/64,
/// 			)
/// 		case sfnt.SegmentOpLineTo:
/// 			r.LineTo(
/// 				originX+float32(seg.Args[0].X)/64,
/// 				originY+float32(seg.Args[0].Y)/64,
/// 			)
/// 		case sfnt.SegmentOpQuadTo:
/// 			r.QuadTo(
/// 				originX+float32(seg.Args[0].X)/64,
/// 				originY+float32(seg.Args[0].Y)/64,
/// 				originX+float32(seg.Args[1].X)/64,
/// 				originY+float32(seg.Args[1].Y)/64,
/// 			)
/// 		case sfnt.SegmentOpCubeTo:
/// 			r.CubeTo(
/// 				originX+float32(seg.Args[0].X)/64,
/// 				originY+float32(seg.Args[0].Y)/64,
/// 				originX+float32(seg.Args[1].X)/64,
/// 				originY+float32(seg.Args[1].Y)/64,
/// 				originX+float32(seg.Args[2].X)/64,
/// 				originY+float32(seg.Args[2].Y)/64,
/// 			)
/// 		}
/// 	}
///
/// 	// Finish the rasterization: the conversion from vector graphics (shapes)
/// 	// to raster graphics (pixels).
/// 	dst := image.NewGray16(image.Rect(0, 0, width, height))
/// 	r.Draw(dst, dst.Bounds(), image.Opaque, image.Point{})
///
/// 	out, _ := os.Create("output.png")
/// 	png.Encode(out, dst)
/// }
