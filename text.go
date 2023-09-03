package main

import (
	"image"
	"image/draw"
	"regexp"
	"strings"

	"github.com/apatters/go-wordwrap"
	"github.com/lukevers/freetype-go/freetype"
	"golang.org/x/image/font/gofont/goregular"
)

const (
	MAX_LINES          = 20
	MAX_CHARS_PER_LINE = 51
	FONT_SIZE          = 7
)

func normalizeText(t string) []string {
	re := regexp.MustCompile(`{div}.*?{/div}`)
	t = re.ReplaceAllString(t, "")
	lines := make([]string, 0, MAX_LINES)
	mention := false
	maxChars := MAX_CHARS_PER_LINE
	for _, line := range strings.Split(t, "\n") {
		line = wordwrap.Wrap(maxChars, line)
		for _, subline := range strings.Split(line, "\n") {
			if strings.HasPrefix(subline, "{blockquote}") {
				mention = true
				subline = strings.ReplaceAll(subline, "{blockquote}", "")
				subline = strings.ReplaceAll(subline, "{/blockquote}", "")
				maxChars = MAX_CHARS_PER_LINE - 1
			} else if strings.HasSuffix(subline, "{/blockquote}") {
				mention = false
				subline = strings.ReplaceAll(subline, "{/blockquote}", "")
				maxChars = MAX_CHARS_PER_LINE
			}
			if mention {
				subline = "> " + subline
			}
			lines = append(lines, subline)
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
	draw.Draw(rgba, rgba.Bounds(), bg, image.ZP, draw.Src)

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
