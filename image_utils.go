package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/png"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode"

	_ "golang.org/x/image/webp"

	"github.com/nfnt/resize"

	"github.com/fogleman/gg"
	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/harfbuzz"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/opentype/api"
	"github.com/go-text/typesetting/shaping"
	"github.com/nbd-wtf/emoji"
	"github.com/pemistahl/lingua-go"
	"github.com/srwiley/rasterx"
	"golang.org/x/exp/slices"
	"golang.org/x/image/math/fixed"
)

const (
	nSupportedScripts = 14
	scaleShift        = 6
)

// highlighting stuff
type hlstate int

const (
	hlNormal  hlstate = 0
	hlLink    hlstate = 1
	hlMention hlstate = 2
	hlHashtag hlstate = 3
)

var (
	supportedScripts = [nSupportedScripts]language.Script{
		language.Unknown,
		language.Latin,
		language.Cyrillic,
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
		"ru",
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

	shaperLock  sync.Mutex
	mainBuffer  = harfbuzz.NewBuffer()
	emojiBuffer = harfbuzz.NewBuffer()
	fontCache   = make(map[font.Face]*harfbuzz.Font)
	emojiFont   *harfbuzz.Font

	lettersAndNumbers = &unicode.RangeTable{}
)

type ScriptRange struct {
	Start  rune
	End    rune
	Pos    int
	Script language.Script
}

func initializeImageDrawingStuff() error {
	// language detector
	if !s.SkipLanguageModel {
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
	}

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
	fontMap[2] = fontMap[0]
	fontMap[3] = loadFont("fonts/NotoSansJP.ttf")
	fontMap[4] = fontMap[3]
	fontMap[5] = loadFont("fonts/NotoSansHebrew.ttf")
	fontMap[6] = loadFont("fonts/NotoSansThai.ttf")
	fontMap[7] = loadFont("fonts/NotoSansArabic.ttf")
	fontMap[8] = loadFont("fonts/NotoSansDevanagari.ttf")
	fontMap[9] = loadFont("fonts/NotoSansBengali.ttf")
	fontMap[10] = loadFont("fonts/NotoSansJavanese.ttf")
	fontMap[11] = loadFont("fonts/NotoSansSC.ttf")
	fontMap[12] = loadFont("fonts/NotoSansKR.ttf")
	emojiFace = loadFont("fonts/NotoEmoji.ttf")

	// shaper stuff
	emojiFont = harfbuzz.NewFont(emojiFace)

	// highlighting stuff
	lettersAndNumbers.LatinOffset = unicode.Latin.LatinOffset + unicode.Number.LatinOffset
	lettersAndNumbers.R16 = make([]unicode.Range16, len(unicode.Latin.R16)+len(unicode.Number.R16)+1)
	copy(lettersAndNumbers.R16, unicode.Latin.R16)
	copy(lettersAndNumbers.R16[len(unicode.Latin.R16):], unicode.Number.R16)
	lettersAndNumbers.R16[len(unicode.Latin.R16)+len(unicode.Number.R16)] = unicode.Range16{
		Lo:     uint16(THIN_SPACE),
		Hi:     uint16(THIN_SPACE),
		Stride: 1,
	}
	slices.SortFunc(lettersAndNumbers.R16, func(a, b unicode.Range16) int { return int(a.Lo) - int(b.Lo) })
	lettersAndNumbers.R32 = make([]unicode.Range32, len(unicode.Latin.R32)+len(unicode.Number.R32))
	copy(lettersAndNumbers.R32, unicode.Latin.R32)
	copy(lettersAndNumbers.R32[len(unicode.Latin.R32):], unicode.Number.R32)
	slices.SortFunc(lettersAndNumbers.R32, func(a, b unicode.Range32) int { return int(a.Lo) - int(b.Lo) })

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
	idx = maxIndex(ranking[2:] /* skip Unknown and Latin because they are the default */)
	idx += 2 // add back the skipped indexes (if maxIndex returns -1 this will default us to 1, latin)

gotScriptIndex:
	script = supportedScripts[idx]
	face = fontMap[idx]
	direction := directionMap[idx]

	lng := language.Language("en-us")
	if detector == nil {
		lng = defaultLanguageMap[idx]
	} else {
		lang, ok := detector.DetectLanguageOf(string(paragraph))
		if ok {
			lng = language.Language(lang.IsoCode639_1().String())
		} else {
			lng = defaultLanguageMap[idx]
		}
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
func shortenURLs(text string, skipImages bool) string {
	return urlMatcher.ReplaceAllStringFunc(text, func(match string) string {
		if skipImages && isMediaURL(match) {
			return match // Skip media URLs
		}

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
// to rely on our dirty globals like fontCache, shaperLock and mainBuffer; it also uses a custom function to
// determine language, script, direction and font face internally instead of taking a shaping.Input argument --
// but also, the most important change was to make it "shape" the same text, twice, with the default font and with
// the emoji font, then build an output of glyphs containing normal glyphs for when the referenced rune is not an
// emoji and an emoji glyph for when it is.
func shapeText(rawText []rune, fontSize int) (shaping.Output, []bool, []hlstate) {
	lang, script, dir, face := getLanguageAndScriptAndDirectionAndFont(rawText)

	shaperLock.Lock()
	defer shaperLock.Unlock()

	// load or get main font from cache
	mainFont, ok := fontCache[face]
	if !ok {
		mainFont = harfbuzz.NewFont(face)
		fontCache[face] = mainFont
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
		{mainFont, mainBuffer},
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
	emojiMask := make([]bool, len(emojiBuffer.Info))

	if len(mainBuffer.Info) > len(emojiBuffer.Info) {
		// remove from mainBuffer characters that are not present in emojiBuffer
		newMainBufferInfo := make([]harfbuzz.GlyphInfo, len(emojiBuffer.Info))
		newMainBufferPos := make([]harfbuzz.GlyphPosition, len(emojiBuffer.Info))
	outer:
		for e, m := 0, 0; e < len(emojiBuffer.Info); {
			ec := emojiBuffer.Info[e].Codepoint
			if ec == mainBuffer.Info[m].Codepoint {
				newMainBufferInfo[e] = mainBuffer.Info[m]
				newMainBufferPos[e] = mainBuffer.Pos[m]

				if emoji.IsEmoji(ec) || emoji.IsTag(ec) || emoji.IsRegionalIndicator(ec) {
					emojiMask[e] = true
				}

				e++
				m++
			} else {
				m++
				for ; ec != mainBuffer.Info[m].Codepoint; m++ {
					// we increase m until mainBuffer catches up with emojiBuffer
					// if we reach the end of mainBuffer and that never happens, then that means it was actually
					// emojiBuffer that had to catch up with mainBuffer -- but we don't handle this for now
					// we just break out of the outer loop and render whatever we had ignoring emojis
					if len(mainBuffer.Info) < m {
						newMainBufferInfo = mainBuffer.Info
						newMainBufferPos = mainBuffer.Pos
						emojiMask = make([]bool, len(emojiBuffer.Info))
						log.Debug().Interface("raw", rawText).Msg("unexpected mismatch between main and emoji buffers")
						break outer
					}
				}
			}
		}
		mainBuffer.Info = newMainBufferInfo
		mainBuffer.Pos = newMainBufferPos
	} else {
		// just go through the glyphs and decide which ones are emojis
		for e := range emojiBuffer.Info {
			ec := emojiBuffer.Info[e].Codepoint
			if emoji.IsEmoji(ec) || emoji.IsTag(ec) || emoji.IsRegionalIndicator(ec) {
				emojiMask[e] = true
			}
		}
	}

	// this will be used to determine if we'll use a different color when rendering a glyph or not
	hlMask := make([]hlstate, len(emojiBuffer.Info))
	var hlState hlstate = hlNormal
	hlSkip := 0 // this will cause us to skip the highlighting parsing phase for the next x glyphs

	// convert the shaped text into an output
	glyphs := make([]shaping.Glyph, len(mainBuffer.Info))
	for i := 0; i < len(glyphs); i++ {

		// deciding if we'll render this as emoji or not
		var buf *harfbuzz.Buffer
		var font *harfbuzz.Font
		if emojiMask[i] {
			buf = emojiBuffer
			font = emojiFont
		} else {
			buf = mainBuffer
			font = mainFont
		}

		// current glyph specs
		glyph := buf.Info[i]

		// naïve text highlighting
		if hlSkip > 0 {
			// skip once
			hlSkip--
		} else {
			switch hlState {
			case hlNormal:
				if glyph.Codepoint == '#' &&
					len(buf.Info) > i+1 &&
					unicode.Is(lettersAndNumbers, buf.Info[i+1].Codepoint) {
					hlState = hlHashtag
					hlSkip = 1 // we already know the next character is a letter in the hashtag, so skip it
				} else if glyph.Codepoint == 'h' &&
					len(buf.Info) > i+1 &&
					buf.Info[i+1].Codepoint == 't' &&
					buf.Info[i+2].Codepoint == 't' &&
					buf.Info[i+3].Codepoint == 'p' {

					if buf.Info[i+4].Codepoint == 's' &&
						buf.Info[i+5].Codepoint == ':' &&
						buf.Info[i+6].Codepoint == '/' &&
						buf.Info[i+7].Codepoint == '/' &&
						buf.Info[i+8].Codepoint != ' ' {
						hlState = hlLink
						hlSkip = 8 // we already know the next 8 characters are 'ttps://_', so skip them
					} else if buf.Info[i+4].Codepoint == ':' &&
						buf.Info[i+5].Codepoint == '/' &&
						buf.Info[i+6].Codepoint == '/' &&
						buf.Info[i+7].Codepoint != ' ' {
						hlState = hlLink
						hlSkip = 7 // we already know the next 8 characters are 'ttp://_', so skip them
					}
				} else if glyph.Codepoint == INVISIBLE_SPACE &&
					len(buf.Info) > i+1 &&
					(unicode.Is(lettersAndNumbers, buf.Info[i+1].Codepoint) || emojiMask[i+1]) {
					hlState = hlMention
					hlSkip = 1 // we already know the next character is a letter or emoji
				}
			case hlLink:
				if glyph.Codepoint == ' ' ||
					glyph.Codepoint == ',' {
					hlState = hlNormal
				}
			case hlMention:
				if !unicode.Is(lettersAndNumbers, glyph.Codepoint) && !emojiMask[i] {
					hlState = hlNormal
				}
			case hlHashtag:
				if !unicode.Is(lettersAndNumbers, glyph.Codepoint) {
					hlState = hlNormal
				}
			}
		}
		hlMask[i] = hlState
		// ~

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

	fontExtents := mainFont.ExtentsForDirection(out.Direction.Harfbuzz())
	out.LineBounds = shaping.Bounds{
		Ascent:  fixed.I(int(fontExtents.Ascender)) >> scaleShift,
		Descent: fixed.I(int(fontExtents.Descender)) >> scaleShift,
		Gap:     fixed.I(int(fontExtents.LineGap)) >> scaleShift,
	}
	out.RecalculateAll()

	return out, emojiMask, hlMask
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
func drawShapedBlockAt(
	img draw.Image,
	fontSize int,
	colors [4]color.Color,
	out shaping.Output,
	emojiMask []bool,
	hlMask []hlstate,
	maskBaseIndex int,
	startX,
	startY int,
) (charsWritten int, endingX int) {
	scale := float32(fontSize) / float32(out.Face.Upem())

	b := img.Bounds()

	var fillers [4]*rasterx.Filler
	for i := range fillers {
		scanner := rasterx.NewScannerGV(b.Dx(), b.Dy(), img, b)
		fillers[i] = rasterx.NewFiller(b.Dx(), b.Dy(), scanner)
		fillers[i].SetColor(colors[i])
	}

	x := float32(startX)
	y := float32(startY)

	for i, g := range out.Glyphs {
		xPos := x + fixed266ToFloat(g.XOffset)
		yPos := y - fixed266ToFloat(g.YOffset)

		face := out.Face
		currentScale := scale
		if emojiMask[maskBaseIndex+i] {
			face = emojiFace
			currentScale = float32(fontSize) / float32(face.Upem())
		}

		f := fillers[hlMask[maskBaseIndex+i]]

		data := face.GlyphData(g.GlyphID)
		switch format := data.(type) {
		case api.GlyphOutline:
			drawOutline(g, format, f, currentScale, xPos, yPos)
		case nil:
			continue
		default:
			panic("format not supported for glyph")
		}

		charsWritten++
		x += fixed266ToFloat(g.XAdvance)
	}

	for _, filler := range fillers {
		filler.Draw()
	}

	return charsWritten, int(math.Ceil(float64(x)))
}

func drawImageAt(img draw.Image, imageUrl string, startY int) int {
	resp, err := http.Get(imageUrl)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()

	srcImg, _, err := image.Decode(resp.Body)
	if err != nil {
		return -1
	}

	// Resize the fetched image to fit the width of the destination image (img)
	width := img.Bounds().Dx()
	resizedImg := resize.Resize(uint(width), 0, srcImg, resize.Lanczos3)
	destY := startY
	destHeight := resizedImg.Bounds().Dy()
	destRect := image.Rect(0, destY, width, destY+destHeight)
	draw.Draw(img, destRect, resizedImg, image.Point{X: 0, Y: 0}, draw.Src)

	return startY + destHeight
}

func drawVideoAt(img draw.Image, videoUrl string, startY int) int {
	tempImagePath := "temp_frame.jpg"
	cmd := exec.Command("ffmpeg", "-i", videoUrl, "-vframes", "1", "-f", "image2", tempImagePath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return -1
	}
	frame, _ := os.Open(tempImagePath)
	defer os.Remove(tempImagePath)
	defer frame.Close()

	imgData, _, err := image.Decode(frame)
	if err != nil {
		return -1
	}

	width := img.Bounds().Dx()
	resizedFrame := resize.Resize(uint(width), 0, imgData, resize.Lanczos3)

	// Draw the play icon on the center of the frame
	videoFrame := image.NewRGBA(resizedFrame.Bounds())
	draw.Draw(videoFrame, videoFrame.Bounds(), resizedFrame, image.Point{}, draw.Src)
	iconFile, _ := static.ReadFile("static/play.png")
	stampImg, _ := png.Decode(bytes.NewBuffer(iconFile))
	videoWidth := videoFrame.Bounds().Dx()
	videoHeight := videoFrame.Bounds().Dy()
	iconWidth := stampImg.Bounds().Dx()
	iconHeight := stampImg.Bounds().Dy()
	posX := (videoWidth - iconWidth) / 2
	posY := (videoHeight - iconHeight) / 2
	destRect := image.Rect(posX, posY, posX+iconWidth, posY+iconHeight)
	draw.Draw(videoFrame, destRect, stampImg, image.Point{}, draw.Over)

	// Draw the modified video frame onto the main canvas
	destRect = image.Rect(0, startY, img.Bounds().Dx(), startY+videoFrame.Bounds().Dy())
	draw.Draw(img, destRect, videoFrame, image.Point{}, draw.Src)

	return startY + videoFrame.Bounds().Dy()
}

func drawMediaAt(img draw.Image, mediaUrl string, startY int) int {
	if isImageURL(mediaUrl) {
		return drawImageAt(img, mediaUrl, startY)
	} else if isVideoURL(mediaUrl) {
		return drawVideoAt(img, mediaUrl, startY)
	} else {
		return startY
	}
}

func isImageURL(input string) bool {
	trimmedURL := strings.TrimSpace(input)
	if trimmedURL == "" {
		return false
	}

	parsedURL, err := url.Parse(trimmedURL)
	if err != nil {
		return false // Unable to parse URL, consider it non-image URL
	}

	// Extract the path (excluding query string and hash fragment)
	path := parsedURL.Path
	imageExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp"}
	for _, ext := range imageExtensions {
		if strings.HasSuffix(strings.ToLower(path), ext) {
			return true // URL points to a valid image
		}
	}
	return false
}

func isVideoURL(input string) bool {
	trimmedURL := strings.TrimSpace(input)
	if trimmedURL == "" {
		return false
	}

	parsedURL, err := url.Parse(trimmedURL)
	if err != nil {
		return false // Unable to parse URL, consider it non-image URL
	}

	// Extract the path (excluding query string and hash fragment)
	path := parsedURL.Path
	imageExtensions := []string{".mp4", ".mov"}
	for _, ext := range imageExtensions {
		if strings.HasSuffix(strings.ToLower(path), ext) {
			return true // URL points to a valid image
		}
	}
	return false
}

func isMediaURL(input string) bool {
	return isImageURL(input) || isVideoURL(input)
}

func containsMedia(paragraphs []string) bool {
	for _, paragraph := range paragraphs {
		if isMediaURL(paragraph) {
			return true
		}
	}
	return false
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
