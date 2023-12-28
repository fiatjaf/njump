package main

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/fogleman/gg"
	"github.com/go-text/typesetting/shaping"
	"github.com/golang/freetype/truetype"
	sdk "github.com/nbd-wtf/nostr-sdk"
	"github.com/nfnt/resize"
	xfont "golang.org/x/image/font"
)

const (
	BLOCK = "|"
)

var (
	BACKGROUND     = color.RGBA{23, 23, 23, 255}
	BAR_BACKGROUND = color.RGBA{10, 10, 10, 255}
	FOREGROUND     = color.RGBA{255, 230, 238, 255}
)

//go:embed fonts/*
var fonts embed.FS

var multiNewlineRe = regexp.MustCompile(`\n\n+`)

func renderImage(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path, ":~", r.Header.Get("user-agent"))

	code := r.URL.Path[1+len("njump/image/"):]
	if code == "" {
		fmt.Fprintf(w, "call /njump/image/<nip19 code>")
		return
	}

	data, err := grabData(r.Context(), code, false)
	if err != nil {
		http.Error(w, "error fetching event: "+err.Error(), 404)
		return
	}

	content := data.event.Content
	content = strings.Replace(content, "\r\n", "\n", -1)
	content = multiNewlineRe.ReplaceAllString(content, "\n\n")
	content = strings.Replace(content, "\t", "  ", -1)
	content = strings.Replace(content, "\r", "", -1)
	content = shortenURLs(content)

	// this turns the raw event.Content into a series of lines ready to drawn
	paragraphs := replaceUserReferencesWithNames(r.Context(),
		quotesAsBlockPrefixedText(r.Context(),
			strings.Split(content, "\n"),
		),
	)

	img, err := drawImage(paragraphs, getPreviewStyle(r), data.metadata, data.createdAt)
	if err != nil {
		log.Printf("error writing image: %s", err)
		http.Error(w, "error writing image!", 500)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "max-age=604800")

	if err := png.Encode(w, img); err != nil {
		log.Printf("error encoding image: %s", err)
		return
	}
}

func drawImage(paragraphs []string, style Style, metadata sdk.ProfileMetadata, date string) (image.Image, error) {
	width := 700
	height := 525
	paddingLeft := 25
	barExtraPadding := 0
	gradientRectHeight := 140
	switch style {
	case StyleTelegram:
		paddingLeft += 10
		width -= 10
	case StyleTwitter:
		height = width * 268 / 512
		barExtraPadding = 105
	}

	img := gg.NewContext(width, height)
	img.SetColor(BACKGROUND)
	img.Clear()
	img.SetColor(FOREGROUND)

	// main content text
	textImg := drawText(paragraphs, width-paddingLeft*2, height-20, true)
	img.DrawImage(textImg, paddingLeft, 20)

	// font for writing the bottom bar stuff
	fontData, _ := fonts.ReadFile("fonts/NotoSans.ttf")
	ttf, _ := truetype.Parse(fontData)
	img.SetFontFace(truetype.NewFace(ttf, &truetype.Options{
		Size:    6,
		DPI:     260,
		Hinting: xfont.HintingFull,
	}))

	// black bar at the bottom
	barHeight := 70
	img.SetColor(BAR_BACKGROUND)
	img.DrawRectangle(0, float64(height-barHeight), float64(width), float64(barHeight))
	img.Fill()

	// a rectangle at the bottom with a gradient from black to transparent
	if len(strings.Join(paragraphs, "\n")) > 141 {
		gradientRectY := height - barHeight - gradientRectHeight
		for y := 0; y < gradientRectHeight; y++ {
			alpha := uint8(255 * (math.Pow(float64(y)/float64(gradientRectHeight), 2)))
			img.SetRGBA255(int(BACKGROUND.R), int(BACKGROUND.G), int(BACKGROUND.B), int(alpha))
			img.DrawRectangle(0, float64(gradientRectY+y), float64(width), 1)
			img.Fill()
		}
	}

	// draw author's name
	authorTextX := paddingLeft + barExtraPadding
	if metadata.Picture != "" {
		authorImage, err := fetchImageFromURL(metadata.Picture)
		if err == nil {
			resizedAuthorImage := resize.Resize(uint(barHeight-20), uint(barHeight-20), roundImage(cropToSquare(authorImage)), resize.Lanczos3)
			img.DrawImage(resizedAuthorImage, paddingLeft+barExtraPadding, height-barHeight+10)
			authorTextX += 65
		}
	}
	authorTextY := height - barHeight + 15
	authorMaxWidth := width/2.0 - paddingLeft*2 - barExtraPadding
	img.SetColor(color.White)
	textImg = drawText([]string{metadata.ShortName()}, width, barHeight, false)
	img.DrawImage(textImg, authorTextX, authorTextY)

	// a gradient to cover too long names
	img.SetColor(BAR_BACKGROUND)
	img.DrawRectangle(float64(authorTextX+authorMaxWidth), float64(height-barHeight), float64(width-authorTextX-authorMaxWidth), float64(barHeight))
	gradientLenght := 60
	for x := 0; x < gradientLenght; x++ {
		alpha := uint8(255 - 255*(math.Pow(float64(x)/float64(gradientLenght), 2)))
		img.SetRGBA255(int(BAR_BACKGROUND.R), int(BAR_BACKGROUND.G), int(BAR_BACKGROUND.B), int(alpha))
		img.DrawRectangle(float64(authorTextX+authorMaxWidth-x), float64(height-barHeight), 1, float64(barHeight))
		img.Fill()
	}

	// bottom bar logo
	logo, _ := static.ReadFile("static/logo.png")
	stampImg, _ := png.Decode(bytes.NewBuffer(logo))
	stampWidth := stampImg.Bounds().Dx()
	stampHeight := stampImg.Bounds().Dy()
	stampX := width - stampWidth - paddingLeft
	stampY := height - stampHeight - 20
	img.DrawImage(stampImg, stampX, stampY)

	// Draw event date
	layout := "2006-01-02 15:04:05"
	parsedTime, _ := time.Parse(layout, date)
	formattedDate := parsedTime.Format("Jan 02, 2006")
	img.SetColor(color.RGBA{160, 160, 160, 255})
	img.DrawStringWrapped(formattedDate, float64(width-paddingLeft-stampWidth-260), float64(height-barHeight+22), 0, 0, float64(240), 1.5, gg.AlignRight)

	return img.Image(), nil
}

func drawText(paragraphs []string, width, height int, dynamicResize bool) image.Image {
	FONT_SIZE := 25
	color := color.RGBA{R: 255, G: 230, B: 238, A: 255}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))

	joinedContent := strings.Join(paragraphs, "\n")
	if dynamicResize && len(joinedContent) < 141 {
		FONT_SIZE = 7
		img := gg.NewContext(width, height)
		fontData, _ := fonts.ReadFile("fonts/NotoSans.ttf")
		ttf, _ := truetype.Parse(fontData)
		i := 1
		lineSpacing := 1.2
		for i < 20 {
			FONT_SIZE += i
			img.SetFontFace(truetype.NewFace(ttf, &truetype.Options{
				Size: float64(FONT_SIZE),
				DPI:  260,
			}))
			wrappedContent := strings.Join(img.WordWrap(joinedContent, float64(width-120)), "\n")
			_, checkHeight := img.MeasureMultilineString(wrappedContent, lineSpacing)
			if checkHeight > float64(height-70-60*2) {
				FONT_SIZE -= 1
				break
			}
			i += 1
		}
		FONT_SIZE = FONT_SIZE*4 - 2
	}

	lineNumber := 1
	for _, paragraph := range paragraphs {
		rawText := []rune(paragraph)

		shapedRunes, emojiMask := shapeText(rawText, FONT_SIZE)

		var wrapper shaping.LineWrapper
		it := shaping.NewSliceIterator([]shaping.Output{shapedRunes})
		lines, _ := wrapper.WrapParagraph(shaping.WrapConfig{}, width, rawText, it)

		totalCharsWritten := 0
		for _, line := range lines {
			for _, out := range line {
				charsWritten, _ := drawShapedRunAt(
					img,
					FONT_SIZE,
					color,
					out,
					emojiMask,
					totalCharsWritten,
					0,
					FONT_SIZE*lineNumber*12/10,
				)
				totalCharsWritten += charsWritten
				lineNumber++
			}
		}
	}

	return img
}
