package main

import (
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

type OpengraphFields struct {
	TwitterCard        string
	TwitterTitle       string
	Superscript        string
	Subscript          string
	Image              string
	Video              string
	VideoFullType      string
	Text               string
	TelegramAndroidApp string
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func TestHomePage(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	renderEvent(w, r)
	if w.Code != 200 {
		t.Fatal("homepage is not 200")
	}
	if !strings.Contains(w.Body.String(), "<form") {
		fmt.Println(w.Body.String())
		t.Fatal("homepage doesn't contain a form")
	}
}

func TestNormalShortTextNote(t *testing.T) {
	og := makeRequest(t, "/nevent1qqswgl5fgcwcrhzmy2u9d3nq6dg7xnsp95657xe0xk9xh6xac43vwsqpqqrf49qe", "")

	assert.Equal(t, og.Image, "", "")
	assert.Equal(t, og.TwitterCard, "summary", "")
	assert.Contains(t, og.Text, "Awesome, good feedback.", "")
}

func TestNoteWithTextImage(t *testing.T) {
	og := makeRequest(t, "/nevent1qqs860kwt3m500hfnve6vxdpagkfqkm6hq03dnn2n7u8dev580kd2uszyztuwzjyxe4x2dwpgken87tna2rdlhpd02va5cvvgrrywpddnr3jydc2w4t", "")

	assert.Contains(t, og.Image, "/image/nevent1qqs860kwt3m500hfnve6vxdpagkfqkm6hq03dnn2n7u8dev580kd2uszyztuwzjyxe4x2dwpgken87tna2rdlhpd02va5cvvgrrywpddnr3jydc2w4t", "")
	assert.Equal(t, og.TwitterCard, "summary_large_image", "")
	assert.Contains(t, og.Text, "seen on", "")
}

func TestNoteAsTelegramInstantView(t *testing.T) {
	og := makeRequest(t, "/nevent1qqs860kwt3m500hfnve6vxdpagkfqkm6hq03dnn2n7u8dev580kd2uszyztuwzjyxe4x2dwpgken87tna2rdlhpd02va5cvvgrrywpddnr3jydc2w4t", "TelegramBot (like TwitterBot)")
	assert.Equal(t, og.TelegramAndroidApp, "Medium", "")
}

func makeRequest(t *testing.T, path string, ua string) *OpengraphFields {
	r := httptest.NewRequest("GET", path, nil)
	r.Header.Set("user-agent", ua)

	w := httptest.NewRecorder()
	renderEvent(w, r)

	if w.Code != 200 {
		t.Fatal("short note is not 200")
	}

	og := &OpengraphFields{}
	parseHead(w.Body, og)

	return og
}

func parseHead(resp io.Reader, og *OpengraphFields) error {
	doc, err := goquery.NewDocumentFromReader(resp)
	if err != nil {
		return fmt.Errorf("failed to parse response with goquery: %w", err)
	}

	doc.Find(`meta[name="twitter:card"]`).Each(func(_ int, s *goquery.Selection) {
		og.TwitterCard, _ = s.Attr("content")
	})
	doc.Find(`meta[name="twitter:title"]`).Each(func(_ int, s *goquery.Selection) {
		og.TwitterTitle, _ = s.Attr("content")
	})
	doc.Find(`meta[property="og:site_name"]`).Each(func(_ int, s *goquery.Selection) {
		og.Superscript, _ = s.Attr("content")
	})
	doc.Find(`meta[property="og:title"]`).Each(func(_ int, s *goquery.Selection) {
		og.Subscript, _ = s.Attr("content")
	})
	doc.Find(`meta[property="og:image"]`).Each(func(_ int, s *goquery.Selection) {
		og.Image, _ = s.Attr("content")
	})
	doc.Find(`meta[property="og:video"]`).Each(func(_ int, s *goquery.Selection) {
		og.Video, _ = s.Attr("content")
	})
	doc.Find(`meta[property="og:video:type"]`).Each(func(_ int, s *goquery.Selection) {
		og.VideoFullType, _ = s.Attr("content")
	})
	doc.Find(`meta[property="og:description"]`).Each(func(_ int, s *goquery.Selection) {
		og.Text, _ = s.Attr("content")
	})
	doc.Find(`meta[property="al:android:app_name"]`).Each(func(_ int, s *goquery.Selection) {
		og.TelegramAndroidApp, _ = s.Attr("content")
	})

	return nil
}
