package main

import (
	"image"
	"image/draw"
	"strings"

	"github.com/apatters/go-wordwrap"
	"github.com/lukevers/freetype-go/freetype"
	"golang.org/x/image/font/gofont/goregular"
)

const (
	MAX_LINES                = 20
	MAX_CHARS_PER_LINE       = 52
	MAX_CHARS_PER_QUOTE_LINE = 48
	FONT_SIZE                = 7
)

func normalizeText(input []string) []string {
	lines := make([]string, 0, MAX_LINES)
	l := 0 // global line counter

	for _, block := range input {
		quoting := false
		maxChars := MAX_CHARS_PER_LINE
		if strings.HasPrefix(block, "> ") {
			quoting = true
			maxChars = MAX_CHARS_PER_QUOTE_LINE // on quote lines we tolerate less characters
			block = block[2:]
			lines = append(lines, "") // add an empty line before each quote
			l++
		}
		for _, line := range strings.Split(block, "\n") {
			if l == MAX_LINES {
				// escape and return here if we're over max lines
				return lines
			}

			line = wordwrap.Wrap(maxChars, strings.TrimSpace(line))
			for _, subline := range strings.Split(line, "\n") {
				// if a line has a word so big that it would overflow (like a nevent), hide it with an ellipsis
				if len(subline) > maxChars {
					subline = subline[0:maxChars-1] + "…"
				}
				if quoting {
					subline = "> " + subline
				}

				lines = append(lines, subline)
				l++
			}
		}
	}
	return lines
}

func drawImage(lines []string, style string) (image.Image, error) {
	width, height, paddingLeft := 700, 525, 0
	switch style {
	case "twitter":
		height = 366
	case "telegram":
		paddingLeft = 15
	}

	// get the physical image ready with colors/size
	fg, bg := image.Black, image.White
	rgba := image.NewRGBA(image.Rect(0, 0, width, height))

	// draw the empty image
	draw.Draw(rgba, rgba.Bounds(), bg, image.Point{}, draw.Src)

	// create new freetype context to get ready for
	// adding text.
	font, _ := freetype.ParseFont(goregular.TTF)

	c := freetype.NewContext()
	c.SetDPI(300)
	c.SetFont(font)
	c.SetFontSize(FONT_SIZE)
	c.SetClip(rgba.Bounds())
	c.SetDst(rgba)
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

	return rgba, nil
}

func drawText(c *freetype.Context, text string, line float64, paddingLeft int) error {
	// We need an offset because we need to know where exactly on the
	// image to place the text. The `line` is how much of an offset
	// that we need to provide (which line the text is going on).
	offsetY := 10 + int(c.PointToFix32(FONT_SIZE*line)>>8)

	_, err := c.DrawString(text, freetype.Pt(10+paddingLeft, offsetY))
	return err
}
