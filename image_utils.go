package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/fiatjaf/emoji"
	"github.com/fogleman/gg"
	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/harfbuzz"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/opentype/api"
	"github.com/go-text/typesetting/shaping"
	"github.com/pemistahl/lingua-go"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/math/fixed"
)

const (
	nSupportedScripts = 13
	scaleShift        = 6
)

var (
	supportedScripts = [nSupportedScripts]language.Script{
		language.Unknown,
		language.Latin,
		language.Hiragana,
		language.Katakana,
		language.Hebrew,
		language.Thai,
		language.Arabic,
		language.Devanagari,
		language.Bengali,
		language.Javanese,
		language.Han,
		language.Hangul,
		language.Syriac,
	}

	detector     lingua.LanguageDetector
	scriptRanges []ScriptRange
	fontMap      [nSupportedScripts]font.Face
	emojiFace    font.Face

	defaultLanguageMap = [nSupportedScripts]language.Language{
		"en-us",
		"en-us",
		"ja",
		"ja",
		"he",
		"th",
		"ar",
		"hi",
		"bn",
		"jv",
		"zh",
		"ko",
		"syr",
	}

	directionMap = [nSupportedScripts]di.Direction{
		di.DirectionLTR,
		di.DirectionLTR,
		di.DirectionLTR,
		di.DirectionLTR,
		di.DirectionRTL,
		di.DirectionLTR,
		di.DirectionRTL,
		di.DirectionLTR,
		di.DirectionLTR,
		di.DirectionLTR,
		di.DirectionLTR,
		di.DirectionLTR,
		di.DirectionRTL,
	}

	shaperLock   sync.Mutex
	shaperBuffer = harfbuzz.NewBuffer()
	emojiBuffer  = harfbuzz.NewBuffer()
	fontCache    = make(map[font.Face]*harfbuzz.Font)
	emojiFont    *harfbuzz.Font
)

type ScriptRange struct {
	Start  rune
	End    rune
	Pos    int
	Script language.Script
}

func initializeImageDrawingStuff() error {
	// language detector
	detector = lingua.NewLanguageDetectorBuilder().FromLanguages(
		lingua.Japanese,
		lingua.Persian,
		lingua.Chinese,
		lingua.Thai,
		lingua.Hebrew,
		lingua.Arabic,
		lingua.Bengali,
		lingua.Hindi,
		lingua.Korean,
	).WithLowAccuracyMode().Build()

	// script detector material
	for _, srange := range language.ScriptRanges {
		for ssi, script := range supportedScripts {
			if srange.Script == script {
				scriptRanges = append(scriptRanges, ScriptRange{
					Start:  srange.Start,
					End:    srange.End,
					Script: srange.Script,
					Pos:    ssi,
				})
			}
		}
	}

	// fonts
	loadFont := func(filepath string) font.Face {
		fontData, err := fonts.ReadFile(filepath)
		face, err := font.ParseTTF(bytes.NewReader(fontData))
		if err != nil {
			log.Fatal().Err(err).Str("path", filepath).Msg("error loading font on startup")
			return nil
		}
		return face
	}
	fontMap[0] = loadFont("fonts/NotoSans.ttf")
	fontMap[1] = fontMap[0]
	fontMap[2] = loadFont("fonts/NotoSansJP.ttf")
	fontMap[3] = fontMap[1]
	fontMap[4] = loadFont("fonts/NotoSansHebrew.ttf")
	fontMap[5] = loadFont("fonts/NotoSansThai.ttf")
	fontMap[6] = loadFont("fonts/NotoSansArabic.ttf")
	fontMap[7] = loadFont("fonts/NotoSansDevanagari.ttf")
	fontMap[8] = loadFont("fonts/NotoSansBengali.ttf")
	fontMap[9] = loadFont("fonts/NotoSansJavanese.ttf")
	fontMap[10] = loadFont("fonts/NotoSansSC.ttf")
	fontMap[11] = loadFont("fonts/NotoSansKR.ttf")
	emojiFace = loadFont("fonts/NotoEmoji.ttf")

	// shaper stuff
	emojiFont = harfbuzz.NewFont(emojiFace)

	return nil
}

// quotesAsBlockPrefixedText replaces nostr:nevent1... and note with their text, as an extra line
// prefixed by BLOCK this returns a slice of lines
func quotesAsBlockPrefixedText(ctx context.Context, lines []string) []string {
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	blocks := make([]string, 0, len(lines)+7)

	for _, line := range lines {
		matches := nostrNoteNeventMatcher.FindAllStringSubmatchIndex(line, -1)

		if len(matches) == 0 {
			// no matches, just return text as it is
			blocks = append(blocks, line)
			continue
		}

		// one or more matches, return multiple lines
		blocks = append(blocks, line[0:matches[0][0]])
		i := -1 // matches iteration counter
		b := 0  // current block index
		for _, match := range matches {
			i++

			matchText := line[match[0]:match[1]]
			submatch := nostrNoteNeventMatcher.FindStringSubmatch(matchText)
			nip19 := submatch[0][6:]

			event, _, err := getEvent(ctx, nip19, nil)
			if err != nil {
				// error case concat this to previous block
				blocks[b] += matchText
				continue
			}

			// add a new block with the quoted text
			blocks = append(blocks, BLOCK+" "+event.Content)

			// increase block count
			b++
		}
		// add remaining text after the last match
		remainingText := line[matches[i][1]:]
		if strings.TrimSpace(remainingText) != "" {
			blocks = append(blocks, remainingText)
		}
	}

	return blocks
}

func getLanguageAndScriptAndDirectionAndFont(paragraph []rune) (
	language.Language,
	language.Script,
	di.Direction,
	font.Face,
) {
	var ranking [nSupportedScripts]int
	nLetters := len(paragraph)
	threshold := nLetters / 2
	var script language.Script
	var face font.Face
	var idx int
	for l := 0; l < nLetters; l++ {
		rnidx := lookupScript(paragraph[l])
		ranking[rnidx]++
		if idx > 0 && l > threshold && ranking[rnidx] > threshold {
			idx = rnidx
			goto gotScriptIndex
		}
	}
	idx = maxIndex(ranking[:])

gotScriptIndex:
	script = supportedScripts[idx]
	face = fontMap[idx]
	direction := directionMap[idx]

	lng := language.Language("en-us")
	lang, ok := detector.DetectLanguageOf(string(paragraph))
	if ok {
		lng = language.Language(lang.IsoCode639_1().String())
	} else {
		lng = defaultLanguageMap[idx]
	}

	return lng, script, direction, face
}

func fetchImageFromURL(url string) (image.Image, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	img, _, err := image.Decode(response.Body)
	if err != nil {
		return nil, err
	}

	return img, nil
}

func roundImage(img image.Image) image.Image {
	bounds := img.Bounds()
	diameter := math.Min(float64(bounds.Dx()), float64(bounds.Dy()))
	radius := diameter / 2

	// Create a new context for the mask
	mask := gg.NewContext(bounds.Dx(), bounds.Dy())
	mask.SetColor(color.Black) // Set the mask color to fully opaque
	mask.DrawCircle(float64(bounds.Dx())/2, float64(bounds.Dy())/2, radius)
	mask.ClosePath()
	mask.Fill()

	// Apply the circular mask to the original image
	result := image.NewRGBA(bounds)
	maskImg := mask.Image()
	draw.DrawMask(result, bounds, img, image.Point{}, maskImg, image.Point{}, draw.Over)

	return result
}

func cropToSquare(img image.Image) image.Image {
	bounds := img.Bounds()
	size := int(math.Min(float64(bounds.Dx()), float64(bounds.Dy())))
	squareImg := image.NewRGBA(image.Rect(0, 0, size, size))
	for x := 0; x < size; x++ {
		for y := 0; y < size; y++ {
			squareImg.Set(x, y, img.At(x+(bounds.Dx()-size)/2, y+(bounds.Dy()-size)/2))
		}
	}
	return squareImg
}

func lookupScript(r rune) int {
	// binary search
	for i, j := 0, len(scriptRanges); i < j; {
		h := i + (j-i)/2
		entry := scriptRanges[h]
		if r < entry.Start {
			j = h
		} else if entry.End < r {
			i = h + 1
		} else {
			return entry.Pos // position in supportedScripts
		}
	}
	return 0 // unknown
}

// shortenURLs takes a text content and returns the same content, but with all big URLs like https://image.nostr.build/0993112ab590e04b978ad32002005d42c289d43ea70d03dafe9ee99883fb7755.jpg#m=image%2Fjpeg&dim=1361x1148&blurhash=%3B7Jjhw00.l.QEh%3FuIA-pMe00%7EVjXX8x%5DE2xuSgtQcr%5E%2500%3FHxD%24%25%25Ms%2Bt%2B-%3BVZK59a%252MyD%2BV%5BI.8%7Ds%3B%25Lso-oi%5ENINHnjI%3BR*%3DdM%7BX7%25MIUtksn%24LM%7BMySeR%25R*%251M%7DRkv%23RjtjS%239as%3AxDnO%251&x=61be75a3e3e0cc88e7f0e625725d66923fdd777b3b691a1c7072ba494aef188d shortened to something like https://image.nostr.build/.../...7755.jpg
func shortenURLs(text string) string {
	return urlMatcher.ReplaceAllStringFunc(text, func(match string) string {
		if len(match) < 50 {
			return match
		}

		parsed, err := url.Parse(match)
		if err != nil {
			return match
		}

		parsed.Fragment = ""

		if len(parsed.RawQuery) > 10 {
			parsed.RawQuery = ""
		}

		pathParts := strings.Split(parsed.Path, "/")
		nParts := len(pathParts)
		lastPart := pathParts[nParts-1]
		if len(lastPart) > 12 {
			pathParts[nParts-1] = "…" + lastPart[len(lastPart)-11:]
		}
		if nParts > 2 {
			pathParts[1] = "…"
			pathParts[2] = pathParts[nParts-1]
			pathParts = pathParts[0:2]
		}

		parsed.Path = "/////"
		urlStr := parsed.String()
		return strings.Replace(urlStr, "/////", strings.Join(pathParts, "/"), 1)
	})
}

// beware: this is all very hacky and I don't know what I am doing!
// this function is copied from go-text/typesetting/shaping's HarfbuzzShaper and adapted to not require a "class",
// to rely on our dirty globals like fontCache, shaperLock and shaperBuffer; it also uses a custom function to
// determine language, script, direction and font face internally instead of taking a shaping.Input argument --
// but also, the most important change was to make it "shape" the same text, twice, with the default font and with
// the emoji font, then build an output of glyphs containing normal glyphs for when the referenced rune is not an
// emoji and an emoji glyph for when it is.
func shapeText(rawText []rune, fontSize int) (shaping.Output, []bool) {
	lang, script, dir, face := getLanguageAndScriptAndDirectionAndFont(rawText)

	shaperLock.Lock()
	defer shaperLock.Unlock()

	// load or get main font from cache
	hfont, ok := fontCache[face]
	if !ok {
		hfont = harfbuzz.NewFont(face)
		fontCache[face] = hfont
	}

	// define this only once
	input := shaping.Input{
		Text:      rawText,
		RunStart:  0,
		RunEnd:    len(rawText),
		Face:      face,
		Size:      fixed.I(int(fontSize)),
		Script:    script,
		Language:  lang,
		Direction: dir,
	}

	// shape stuff for both normal text and emojis
	for _, params := range []struct {
		font *harfbuzz.Font
		buf  *harfbuzz.Buffer
	}{
		{hfont, shaperBuffer},
		{emojiFont, emojiBuffer},
	} {
		params.buf.Clear() // clear before using

		runes, start, end := input.Text, input.RunStart, input.RunEnd
		if end < start {
			panic("end < start")
		}
		start = clamp(start, 0, len(runes))
		end = clamp(end, 0, len(runes))
		params.buf.AddRunes(runes, start, end-start)

		params.buf.Props.Direction = input.Direction.Harfbuzz()
		params.buf.Props.Language = input.Language
		params.buf.Props.Script = input.Script

		// adjust the user provided fields
		params.font.XScale = int32(input.Size.Ceil()) << scaleShift
		params.font.YScale = params.font.XScale

		// actually use harfbuzz to shape the text.
		params.buf.Shape(params.font, nil)
	}

	// this will be used to determine whether a given glyph is an emoji or not when rendering
	emojiMask := make([]bool, len(shaperBuffer.Info))

	// convert the shaped text into an output
	glyphs := make([]shaping.Glyph, len(shaperBuffer.Info))
	for i := 0; i < len(glyphs); i++ {
		var buf *harfbuzz.Buffer
		var font *harfbuzz.Font
		if i < len(rawText) && emoji.IsEmoji(rawText[i]) {
			buf = emojiBuffer
			font = emojiFont
			emojiMask[i] = true
		} else {
			buf = shaperBuffer
			font = hfont
		}
		glyph := buf.Info[i]

		glyphs[i] = shaping.Glyph{
			ClusterIndex: glyph.Cluster,
			GlyphID:      glyph.Glyph,
			Mask:         glyph.Mask,
		}
		extents, ok := font.GlyphExtents(glyph.Glyph)
		if !ok {
			continue
		}
		glyphs[i].Width = fixed.I(int(extents.Width)) >> scaleShift
		glyphs[i].Height = fixed.I(int(extents.Height)) >> scaleShift
		glyphs[i].XBearing = fixed.I(int(extents.XBearing)) >> scaleShift
		glyphs[i].YBearing = fixed.I(int(extents.YBearing)) >> scaleShift
		glyphs[i].XAdvance = fixed.I(int(buf.Pos[i].XAdvance)) >> scaleShift
		glyphs[i].YAdvance = fixed.I(int(buf.Pos[i].YAdvance)) >> scaleShift
		glyphs[i].XOffset = fixed.I(int(buf.Pos[i].XOffset)) >> scaleShift
		glyphs[i].YOffset = fixed.I(int(buf.Pos[i].YOffset)) >> scaleShift
	}

	countClusters(glyphs, input.RunEnd, input.Direction.Progression())
	out := shaping.Output{
		Glyphs:    glyphs,
		Direction: input.Direction,
		Face:      input.Face,
		Size:      input.Size,
	}
	out.Runes.Offset = input.RunStart
	out.Runes.Count = input.RunEnd - input.RunStart

	fontExtents := hfont.ExtentsForDirection(out.Direction.Harfbuzz())
	out.LineBounds = shaping.Bounds{
		Ascent:  fixed.I(int(fontExtents.Ascender)) >> scaleShift,
		Descent: fixed.I(int(fontExtents.Descender)) >> scaleShift,
		Gap:     fixed.I(int(fontExtents.LineGap)) >> scaleShift,
	}
	out.RecalculateAll()

	return out, emojiMask
}

// this function is copied from go-text/typesetting/shaping because shapeText needs it
func countClusters(glyphs []shaping.Glyph, textLen int, dir di.Progression) {
	currentCluster := -1
	runesInCluster := 0
	glyphsInCluster := 0
	previousCluster := textLen
	for i := range glyphs {
		g := glyphs[i].ClusterIndex
		if g != currentCluster {
			// If we're processing a new cluster, count the runes and glyphs
			// that compose it.
			runesInCluster = 0
			glyphsInCluster = 1
			currentCluster = g
			nextCluster := -1
		glyphCountLoop:
			for k := i + 1; k < len(glyphs); k++ {
				if glyphs[k].ClusterIndex == g {
					glyphsInCluster++
				} else {
					nextCluster = glyphs[k].ClusterIndex
					break glyphCountLoop
				}
			}
			if nextCluster == -1 {
				nextCluster = textLen
			}
			switch dir {
			case di.FromTopLeft:
				runesInCluster = nextCluster - currentCluster
			case di.TowardTopLeft:
				runesInCluster = previousCluster - currentCluster
			}
			previousCluster = g
		}
		glyphs[i].GlyphCount = glyphsInCluster
		glyphs[i].RuneCount = runesInCluster
	}
}

// this function is copied from go-text/render, but adapted to not require a "class" to be instantiated and also,
// more importantly, to take an emojiMask parameter, with the same length as out.Glyphs, to determine when a
// glyph should be rendered with the emoji font instead of with the default font
func drawShapedRunAt(
	fontSize int,
	clr color.Color,
	out shaping.Output,
	emojiMask []bool,
	img draw.Image,
	startX,
	startY int,
) int {
	scale := float32(fontSize) / float32(out.Face.Upem())

	b := img.Bounds()
	scanner := rasterx.NewScannerGV(b.Dx(), b.Dy(), img, b)
	f := rasterx.NewFiller(b.Dx(), b.Dy(), scanner)
	f.SetColor(clr)
	x := float32(startX)
	y := float32(startY)
	for i, g := range out.Glyphs {
		xPos := x + fixed266ToFloat(g.XOffset)
		yPos := y - fixed266ToFloat(g.YOffset)

		face := out.Face
		currentScale := scale
		if emojiMask[i] {
			face = emojiFace
			currentScale = float32(fontSize) / float32(face.Upem())
		}

		data := face.GlyphData(g.GlyphID)
		switch format := data.(type) {
		case api.GlyphOutline:
			drawOutline(g, format, f, currentScale, xPos, yPos)
		case nil:
			continue
		default:
			panic("format not supported for glyph")
		}

		x += fixed266ToFloat(g.XAdvance)
	}
	f.Draw()
	return int(math.Ceil(float64(x)))
}

// this draws a font glyph (i.e. a letter) according to instructions and scale and whatever
func drawOutline(g shaping.Glyph, bitmap api.GlyphOutline, f *rasterx.Filler, scale float32, x, y float32) {
	for _, s := range bitmap.Segments {
		switch s.Op {
		case api.SegmentOpMoveTo:
			f.Start(fixed.Point26_6{X: floatToFixed266(s.Args[0].X*scale + x), Y: floatToFixed266(-s.Args[0].Y*scale + y)})
		case api.SegmentOpLineTo:
			f.Line(fixed.Point26_6{X: floatToFixed266(s.Args[0].X*scale + x), Y: floatToFixed266(-s.Args[0].Y*scale + y)})
		case api.SegmentOpQuadTo:
			f.QuadBezier(fixed.Point26_6{X: floatToFixed266(s.Args[0].X*scale + x), Y: floatToFixed266(-s.Args[0].Y*scale + y)},
				fixed.Point26_6{X: floatToFixed266(s.Args[1].X*scale + x), Y: floatToFixed266(-s.Args[1].Y*scale + y)})
		case api.SegmentOpCubeTo:
			f.CubeBezier(fixed.Point26_6{X: floatToFixed266(s.Args[0].X*scale + x), Y: floatToFixed266(-s.Args[0].Y*scale + y)},
				fixed.Point26_6{X: floatToFixed266(s.Args[1].X*scale + x), Y: floatToFixed266(-s.Args[1].Y*scale + y)},
				fixed.Point26_6{X: floatToFixed266(s.Args[2].X*scale + x), Y: floatToFixed266(-s.Args[2].Y*scale + y)})
		}
	}
	f.Stop(true)
}

func fixed266ToFloat(i fixed.Int26_6) float32 {
	return float32(float64(i) / 64)
}

func floatToFixed266(f float32) fixed.Int26_6 {
	return fixed.Int26_6(int(float64(f) * 64))
}
