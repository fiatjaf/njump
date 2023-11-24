package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/apatters/go-wordwrap"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

const (
	MAX_LINES                = 20
	MAX_CHARS_PER_LINE       = 52
	MAX_CHARS_PER_QUOTE_LINE = 48
	FONT_SIZE                = 7
	FONT_DPI                 = 300

	BLOCK = "|"
)

var (
	BACKGROUND = color.RGBA{23, 23, 23, 255}
	FOREGROUND = color.RGBA{255, 230, 238, 255}
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

func drawImage(lines []string, ttf *truetype.Font, style Style) (image.Image, error) {
	width := 700
	height := 525
	paddingLeft := 0
	switch style {
	case StyleTelegram:
		paddingLeft = 15
	case StyleTwitter:
		height = width * 268 / 512
	}

	img := gg.NewContext(width, height)
	img.SetColor(BACKGROUND)
	img.Clear()
	img.SetColor(FOREGROUND)
	img.SetFontFace(truetype.NewFace(ttf, &truetype.Options{
		Size:    FONT_SIZE,
		DPI:     FONT_DPI,
		Hinting: font.HintingFull,
	}))

	lineSpacing := 0.3
	lineHeight := float64(FONT_SIZE)*FONT_DPI/72.0 + float64(FONT_SIZE)*lineSpacing*FONT_DPI/72.0
	for i, line := range lines {
		y := float64(i)*lineHeight + 50                  // Calculate the Y position for each line
		img.DrawString(line, float64(20+paddingLeft), y) // Draw the line at the calculated Y position
	}

	// create the stamp image
	logo, _ := static.ReadFile("static/logo.png")
	stampImg, _ := png.Decode(bytes.NewBuffer(logo))
	stampWidth := stampImg.Bounds().Dx()
	stampHeight := stampImg.Bounds().Dy()

	// Calculate the position for the stamp in the bottom right corner
	stampX := width - stampWidth - 20
	stampY := height - stampHeight - 20

	// Draw the stamp onto the image
	img.DrawImage(stampImg, stampX, stampY)

	return img.Image(), nil
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
			unicode.Han,
			"fonts/NotoSansSC.ttf",
			true,
		},
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

	font, err := truetype.Parse(fontData)
	if err != nil {
		return nil, false, err
	}

	return font, shouldBreakWords, nil
}
