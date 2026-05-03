package gmd

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/lumifloat/tinyskia"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/f64"
	"golang.org/x/image/math/fixed"
)

type tinySkiaDrawBackend struct {
	fonts map[string]*tinySkiaDrawFont
}

func newTinySkiaDrawBackend() *tinySkiaDrawBackend {
	return &tinySkiaDrawBackend{
		fonts: make(map[string]*tinySkiaDrawFont, 32),
	}
}

func (b *tinySkiaDrawBackend) newCanvas(width, height int) drawCanvas {
	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	return &tinySkiaDrawCanvas{
		dc:   tinyskia.NewContextForRGBA(rgba),
		rgba: rgba,
	}
}

func (b *tinySkiaDrawBackend) newFont(fonts *fontManager, family FontFamily, size float64) (drawFont, error) {
	key := string(family) + "|" + strconv.FormatFloat(size, 'f', 2, 64)
	if face, ok := b.fonts[key]; ok {
		return face, nil
	}

	ttf, err := fonts.parsedFont()
	if err != nil {
		return nil, err
	}

	ppem := fixed.Int26_6(size * 64)
	metrics, err := ttf.Metrics(nil, ppem, font.HintingNone)
	if err != nil {
		return nil, fmt.Errorf("read font metrics: %w", err)
	}

	face, err := opentype.NewFace(ttf, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingNone,
	})
	if err != nil {
		return nil, err
	}

	out := &tinySkiaDrawFont{
		face:    face,
		ascent:  fixedToFloat(metrics.Ascent),
		descent: fixedToFloat(metrics.Descent),
	}
	b.fonts[key] = out
	return out, nil
}

type tinySkiaDrawFont struct {
	face    font.Face
	ascent  float64
	descent float64
}

func (f *tinySkiaDrawFont) metrics() (float64, float64) {
	return f.ascent, f.descent
}

func (f *tinySkiaDrawFont) measure(text string) float64 {
	drawer := font.Drawer{Face: f.face}
	return fixedToFloat(drawer.MeasureString(sanitizeTinySkiaText(text)))
}

type tinySkiaDrawCanvas struct {
	dc   *tinyskia.Context
	rgba *image.RGBA
}

func (c *tinySkiaDrawCanvas) Clear(clr color.Color) {
	c.dc.SetFillStyleSolidColor(clr)
	c.dc.FillRect(0, 0, float64(c.dc.Width()), float64(c.dc.Height()))
}

func (c *tinySkiaDrawCanvas) FillRoundedRect(x, y, w, h, radius float64, clr color.Color) {
	c.dc.BeginPath()
	c.dc.SetFillStyleSolidColor(clr)
	c.dc.RoundRect(x, y, w, h, []float64{radius})
	c.dc.Fill()
}

func (c *tinySkiaDrawCanvas) StrokeLine(x1, y1, x2, y2, width float64, clr color.Color) {
	c.dc.BeginPath()
	c.dc.SetStrokeStyleSolidColor(clr)
	c.dc.SetLineWidth(width)
	c.dc.MoveTo(x1, y1)
	c.dc.LineTo(x2, y2)
	c.dc.Stroke()
}

func (c *tinySkiaDrawCanvas) FillCircle(x, y, radius float64, clr color.Color) {
	c.dc.BeginPath()
	c.dc.SetFillStyleSolidColor(clr)
	c.dc.Arc(x, y, radius, 0, math.Pi*2, false)
	c.dc.Fill()
}

func (c *tinySkiaDrawCanvas) DrawText(text string, x, y float64, face drawFont, clr color.Color, fauxItalic bool) {
	fontFace, ok := face.(*tinySkiaDrawFont)
	if !ok {
		panic(fmt.Sprintf("gmd: tinyskia canvas received %T font", face))
	}

	text = sanitizeTinySkiaText(text)
	if text == "" {
		return
	}

	if fauxItalic {
		c.drawItalicText(text, x, y, fontFace, clr)
		return
	}

	drawer := font.Drawer{
		Dst:  c.rgba,
		Src:  image.NewUniform(clr),
		Face: fontFace.face,
		Dot: fixed.Point26_6{
			X: fixed.Int26_6(x * 64),
			Y: fixed.Int26_6(y * 64),
		},
	}
	drawer.DrawString(text)
}

func (c *tinySkiaDrawCanvas) DrawImage(img image.Image, x, y int) {
	c.dc.DrawImage(img, float64(x), float64(y))
}

func (c *tinySkiaDrawCanvas) Image() image.Image {
	return c.dc.Image()
}

func (c *tinySkiaDrawCanvas) drawItalicText(text string, x, y float64, face *tinySkiaDrawFont, clr color.Color) {
	const shear = -0.18

	textWidth := face.measure(text)
	padX := int(math.Ceil(face.ascent*math.Abs(shear))) + 4
	padY := 3
	width := int(math.Ceil(textWidth)) + padX*2 + 4
	height := int(math.Ceil(face.ascent+face.descent)) + padY*2 + 4
	if width <= 0 || height <= 0 {
		return
	}

	src := image.NewRGBA(image.Rect(0, 0, width, height))
	baseline := float64(padY) + face.ascent
	drawer := font.Drawer{
		Dst:  src,
		Src:  image.NewUniform(clr),
		Face: face.face,
		Dot: fixed.Point26_6{
			X: fixed.I(padX),
			Y: fixed.Int26_6(baseline * 64),
		},
	}
	drawer.DrawString(text)

	matrix := f64.Aff3{
		1, shear, x - float64(padX) - shear*baseline,
		0, 1, y - baseline,
	}
	xdraw.ApproxBiLinear.Transform(c.rgba, matrix, src, src.Bounds(), xdraw.Over, nil)
}

func sanitizeTinySkiaText(text string) string {
	if !strings.ContainsAny(text, "\t\r\n") && !hasControlRune(text) {
		return text
	}

	var builder strings.Builder
	builder.Grow(len(text))
	for _, rn := range text {
		switch rn {
		case '\t':
			builder.WriteString("    ")
		case '\r', '\n':
			continue
		default:
			if unicode.IsControl(rn) {
				builder.WriteByte(' ')
			} else {
				builder.WriteRune(rn)
			}
		}
	}
	return builder.String()
}

func hasControlRune(text string) bool {
	for _, rn := range text {
		if unicode.IsControl(rn) {
			return true
		}
	}
	return false
}
