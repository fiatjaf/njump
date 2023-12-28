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

	content := strings.Replace(data.event.Content, "\n\n\n\n", "\n\n", -1)
	content = strings.Replace(data.event.Content, "\n\n\n", "\n\n", -1)
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
	barExtraPadding := 5
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
	textImg := drawText(paragraphs, width-25*2, height-20)
	img.DrawImage(textImg, 25, 20)

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
	gradientRectHeight := 140
	gradientRectY := height - barHeight - gradientRectHeight
	for y := 0; y < gradientRectHeight; y++ {
		alpha := uint8(255 * (math.Pow(float64(y)/float64(gradientRectHeight), 2)))
		img.SetRGBA255(int(BACKGROUND.R), int(BACKGROUND.G), int(BACKGROUND.B), int(alpha))
		img.DrawRectangle(0, float64(gradientRectY+y), float64(width), 1)
		img.Fill()
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
	authorTextY := height - barHeight + 20
	authorMaxWidth := width/2.0 - paddingLeft*2 - barExtraPadding
	img.SetColor(color.White)
	img.DrawStringWrapped(metadata.ShortName(), float64(authorTextX), float64(authorTextY), 0, 0, float64(width*99), 99, gg.AlignLeft)

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
	img.DrawStringWrapped(formattedDate, float64(width-paddingLeft-stampWidth-260), float64(authorTextY+3), 0, 0, float64(240), 1.5, gg.AlignRight)

	return img.Image(), nil
}

func drawText(paragraphs []string, width, height int) image.Image {
	const FONT_SIZE = 25
	color := color.RGBA{R: 255, G: 230, B: 238, A: 255}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))

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
					FONT_SIZE,
					color,
					out,
					emojiMask,
					totalCharsWritten,
					img,
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
