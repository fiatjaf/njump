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

	// Trim fake extensions
	extensions := []string{".png", ".jpg", ".jpeg"}
	for _, ext := range extensions {
		code = strings.TrimSuffix(code, ext)
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
	content = shortenURLs(content, true)

	// this turns the raw event.Content into a series of lines ready to drawn
	paragraphs := replaceUserReferencesWithNames(r.Context(),
		quotesAsBlockPrefixedText(r.Context(),
			strings.Split(content, "\n"),
		),
		string(INVISIBLE_SPACE),
		"",
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

func drawImage(paragraphs []string, style Style, metadata Metadata, date string) (image.Image, error) {
	fontSize := 25
	width := 700
	height := 525
	paddingLeft := 25
	gradientRectHeight := 140
	barHeight := 70
	switch style {
	case StyleTelegram:
		paddingLeft += 10
		width -= 10
	case StyleTwitter:
		height = width * 268 / 512
	}

	img := gg.NewContext(width, height)
	img.SetColor(BACKGROUND)
	img.Clear()
	img.SetColor(FOREGROUND)

	// main content text
	addedSize := 0
	zoom := 1.0
	textFontSize := fontSize
	if np := len(paragraphs); !containsMedia(paragraphs) && np < 6 {
		nchars := 0
		blankLines := 0
		for _, par := range paragraphs {
			nchars += len([]rune(par))
			if par == "" {
				blankLines++
			}
		}
		largeness := math.Pow(float64(nchars), 0.60) + math.Pow(float64(np-blankLines), 1) + math.Pow(float64(blankLines), 0.7)
		zoom = float64(math.Pow(float64(height)/366.0-(float64(blankLines+1)/10), 1.2))
		addedSize = int(200.0 / largeness * zoom)
		textFontSize = int(float64(fontSize + addedSize))
	}
	textImg, overflowingText := drawParagraphs(paragraphs, textFontSize, width-paddingLeft*2, height-20-barHeight)
	img.DrawImage(textImg, paddingLeft, 20)

	// font for writing the date
	fontData, _ := fonts.ReadFile("fonts/NotoSans.ttf")
	ttf, _ := truetype.Parse(fontData)
	img.SetFontFace(truetype.NewFace(ttf, &truetype.Options{
		Size:    6,
		DPI:     260,
		Hinting: xfont.HintingFull,
	}))

	// black bar at the bottom
	img.SetColor(BAR_BACKGROUND)
	img.DrawRectangle(0, float64(height-barHeight), float64(width), float64(barHeight))
	img.Fill()

	// a rectangle at the bottom with a gradient from black to transparent
	if overflowingText {
		gradientRectY := height - barHeight - gradientRectHeight
		for y := 0; y < gradientRectHeight; y++ {
			alpha := uint8(255 * (math.Pow(float64(y)/float64(gradientRectHeight), 2)))
			img.SetRGBA255(int(BACKGROUND.R), int(BACKGROUND.G), int(BACKGROUND.B), int(alpha))
			img.DrawRectangle(0, float64(gradientRectY+y), float64(width), 1)
			img.Fill()
		}
	}

	// draw author's name
	authorTextX := paddingLeft
	if metadata.Picture != "" {
		authorImage, err := fetchImageFromURL(metadata.Picture)
		if err == nil {
			resizedAuthorImage := resize.Resize(uint(barHeight-20), uint(barHeight-20), roundImage(cropToSquare(authorImage)), resize.Lanczos3)
			img.DrawImage(resizedAuthorImage, paddingLeft, height-barHeight+10)
			authorTextX += 65
		}
	}
	authorTextY := height - barHeight + 15
	authorMaxWidth := width/2.0 - paddingLeft*2
	img.SetColor(color.White)
	textImg, _ = drawParagraphs([]string{metadata.ShortName()}, fontSize, width, barHeight)
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

func drawParagraphs(paragraphs []string, fontSize int, width, height int) (image.Image, bool) {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))

	lineNumber := 1
	yPos := fontSize * lineNumber * 12 / 10
	for i := 0; i < len(paragraphs); i++ {
		paragraph := paragraphs[i]

		// Skip empty lines if the next element is an image
		if paragraph == "" && len(paragraphs) > i+1 && isMediaURL(paragraphs[i+1]) {
			continue
		}

		if isMediaURL(paragraph) {
			if i == 0 {
				yPos = 0
			}
			next := drawMediaAt(img, paragraph, yPos)
			if next != -1 {
				yPos = next
				// this means the media picture was successfully drawn
				continue
			}

			// if we reach here that means we didn't draw anything, so proceed to
			// draw the text
		}

		rawText := []rune(paragraph)

		shapedRunes, emojiMask, hlMask := shapeText(rawText, fontSize)

		var wrapper shaping.LineWrapper
		it := shaping.NewSliceIterator([]shaping.Output{shapedRunes})
		lines, _ := wrapper.WrapParagraph(shaping.WrapConfig{}, width, rawText, it)

		totalCharsWritten := 0
		for _, line := range lines {
			for _, out := range line {
				charsWritten, _ := drawShapedBlockAt(
					img,
					fontSize,
					[4]color.Color{
						color.RGBA{R: 255, G: 230, B: 238, A: 255}, // normal
						color.RGBA{R: 242, G: 211, B: 152, A: 255}, // links
						color.RGBA{R: 227, G: 42, B: 109, A: 255},  // mentions -> Tailwind strongpink
						color.RGBA{R: 151, G: 210, B: 251, A: 255}, // hashtags
					},
					out,
					emojiMask,
					hlMask,
					totalCharsWritten,
					0,
					yPos,
				)
				totalCharsWritten += charsWritten

				if fontSize*lineNumber*12/10 > height {
					return img, true
				}
				lineNumber++
				yPos = yPos + fontSize*12/10
			}
		}
	}

	return img, false
}
