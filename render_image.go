package main

import (
	"context"
	"embed"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/apatters/go-wordwrap"
	"github.com/lukevers/freetype-go/freetype"
	"github.com/lukevers/freetype-go/freetype/truetype"
)

const (
	MAX_LINES                = 20
	MAX_CHARS_PER_LINE       = 52
	MAX_CHARS_PER_QUOTE_LINE = 48
	FONT_SIZE                = 7

	BLOCK = "▒"
)

//go:embed fonts/*
var fonts embed.FS

func renderImage(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path, ":~", r.Header.Get("user-agent"))

	code := r.URL.Path[1+len("njump/image/"):]
	if code == "" {
		fmt.Fprintf(w, "call /njump/image/<nip19 code>")
		return
	}

	event, _, err := getEvent(r.Context(), code, nil)
	if err != nil {
		http.Error(w, "error fetching event: "+err.Error(), 404)
		return
	}

	// get the font and language specifics based on the characters used
	font, breakWords, err := getLanguage(event.Content)
	if err != nil {
		http.Error(w, "error getting font: "+err.Error(), 500)
		return
	}

	// this turns the raw event.Content into a series of lines ready to drawn
	lines := normalizeText(
		replaceUserReferencesWithNames(r.Context(),
			renderQuotesAsBlockPrefixedText(r.Context(),
				event.Content,
			),
		),
		breakWords,
	)

	img, err := drawImage(lines, font, getPreviewStyle(r))
	if err != nil {
		log.Printf("error writing image: %s", err)
		http.Error(w, "error writing image!", 500)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "max-age=604800")

	if err := png.Encode(w, img); err != nil {
		log.Printf("error encoding image: %s", err)
		http.Error(w, "error encoding image!", 500)
		return
	}
}

func normalizeText(input []string, breakWords bool) []string {
	lines := make([]string, 0, MAX_LINES)
	l := 0 // global line counter

	for _, block := range input {
		quoting := false
		maxChars := MAX_CHARS_PER_LINE
		if strings.HasPrefix(block, BLOCK+" ") {
			quoting = true
			maxChars = MAX_CHARS_PER_QUOTE_LINE // on quote lines we tolerate less characters
			block = block[len(BLOCK)+1:]
			lines = append(lines, "") // add an empty line before each quote
			l++
		}
		for _, line := range strings.Split(block, "\n") {
			if l == MAX_LINES {
				// escape and return here if we're over max lines
				return lines
			}

			// turn a single line into multiple if it is long enough -- carefully splitting on word ends
			wrappedLines := strings.Split(wordwrap.Wrap(maxChars, strings.TrimSpace(line)), "\n")

			// now we go over all these lines and further split them if necessary
			// in japanese, for example, we must break the words otherwise nothing works
			var sublines []string
			if breakWords {
				sublines = make([]string, 0, len(wrappedLines))
				for _, wline := range wrappedLines {
					// split until we have a bunch of lines all under maxChars
					for {
						if len(wline) > maxChars {
							// we can't split exactly at maxChars because that would break utf-8 runes
							// so we do this range mess to try to grab where the last rune in the line ends
							subline := make([]rune, 0, maxChars)
							var i int
							var r rune
							for i, r = range wline {
								if i > maxChars {
									break
								}
								subline = append(subline, r)
							}
							sublines = append(sublines, string(subline))
							wline = wline[i:]
						} else {
							sublines = append(sublines, wline)
							break
						}
					}
				}
			} else {
				sublines = wrappedLines
			}

			for _, subline := range sublines {
				// if a line has a word so big that it would overflow (like a nevent), hide it with an ellipsis
				if len([]rune(subline)) > maxChars {
					subline = subline[0:maxChars-1] + "…"
				}

				if quoting {
					subline = BLOCK + " " + subline
				}

				lines = append(lines, subline)
				l++
			}
		}
	}
	return lines
}

func drawImage(lines []string, font *truetype.Font, style string) (image.Image, error) {
	width := 700
	height := 525
	paddingLeft := 0
	switch style {
	case "telegram":
		paddingLeft = 15
	case "twitter":
		height = width * 268 / 512
	}

	// get the physical image ready with colors/size
	fg, bg := image.Black, image.White
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// draw the empty image
	draw.Draw(img, img.Bounds(), bg, image.Point{}, draw.Src)

	// create new freetype context to get ready for adding text.
	c := freetype.NewContext()
	c.SetDPI(300)
	c.SetFont(font)
	c.SetFontSize(FONT_SIZE)
	c.SetClip(img.Bounds())
	c.SetDst(img)
	c.SetSrc(fg)
	c.SetHinting(freetype.NoHinting)

	// draw each line separately
	var count float64 = 1
	for _, line := range lines {
		if err := drawText(c, line, count, paddingLeft); err != nil {
			return nil, err
		}
		count++
	}

	return img, nil
}

func drawText(c *freetype.Context, text string, line float64, paddingLeft int) error {
	// We need an offset because we need to know where exactly on the
	// image to place the text. The `line` is how much of an offset
	// that we need to provide (which line the text is going on).
	offsetY := 10 + int(c.PointToFix32(FONT_SIZE*line)>>8)

	_, err := c.DrawString(text, freetype.Pt(10+paddingLeft, offsetY))
	return err
}

// replace nevent and note with their text, as an extra line prefixed by BLOCK
// this returns a slice of lines
func renderQuotesAsBlockPrefixedText(ctx context.Context, input string) []string {
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	blocks := make([]string, 0, 8)
	matches := nostrNoteNeventMatcher.FindAllStringSubmatchIndex(input, -1)

	if len(matches) == 0 {
		// no matches, just return text as it is
		blocks = append(blocks, input)
		return blocks
	}

	// one or more matches, return multiple lines
	blocks = append(blocks, input[0:matches[0][0]])
	i := -1 // matches iteration counter
	b := 0  // current block index
	for _, match := range matches {
		i++

		matchText := input[match[0]:match[1]]
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
	remainingText := input[matches[i][1]:]
	if strings.TrimSpace(remainingText) != "" {
		blocks = append(blocks, remainingText)
	}

	return blocks
}

func getLanguage(text string) (*truetype.Font, bool, error) {
	fontName := "fonts/NotoSans.ttf"
	shouldBreakWords := false

	for _, group := range []struct {
		lang       *unicode.RangeTable
		fontName   string
		breakWords bool
	}{
		{
			unicode.Katakana,
			"fonts/NotoSansJP.ttf",
			true,
		},
		{
			unicode.Hiragana,
			"fonts/NotoSansJP.ttf",
			true,
		},
		{
			unicode.Han,
			"fonts/NotoSansTC.ttf",
			true,
		},
		{
			unicode.Hangul,
			"fonts/NotoSansKR.ttf",
			true,
		},
		{
			unicode.Arabic,
			"fonts/NotoSansArabic.ttf",
			false,
		},
		{
			unicode.Hebrew,
			"fonts/NotoSansHebrew.ttf",
			false,
		},
		{
			unicode.Bengali,
			"fonts/NotoSansBengali.ttf",
			false,
		},
		{
			unicode.Thai,
			"fonts/NotoSansThai.ttf",
			false,
		},
	} {
		for _, rune := range text {
			rune16 := uint16(rune)
			for _, r16 := range group.lang.R16 {
				if rune16 >= r16.Lo && rune16 <= r16.Hi {
					fontName = group.fontName
					shouldBreakWords = group.breakWords
					goto gotLang
				}
			}
			rune32 := uint32(rune)
			for _, r32 := range group.lang.R32 {
				if rune32 >= r32.Lo && rune32 <= r32.Hi {
					fontName = group.fontName
					shouldBreakWords = group.breakWords
					goto gotLang
				}
			}
		}
	}

gotLang:
	fontData, err := fonts.ReadFile(fontName)
	if err != nil {
		return nil, false, err
	}

	font, err := freetype.ParseFont(fontData)
	if err != nil {
		return nil, false, err
	}

	return font, shouldBreakWords, nil
}
