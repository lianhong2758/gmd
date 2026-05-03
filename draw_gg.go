package gmd

import (
	"fmt"
	"image"
	"image/color"
	"strconv"

	"github.com/FloatTech/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

type ggDrawBackend struct {
	measureDC *gg.Context
	fonts     map[string]*ggDrawFont
}

func newGGDrawBackend() *ggDrawBackend {
	return &ggDrawBackend{
		measureDC: gg.NewContext(8, 8),
		fonts:     make(map[string]*ggDrawFont, 32),
	}
}

func (b *ggDrawBackend) newCanvas(width, height int) drawCanvas {
	return &ggDrawCanvas{dc: gg.NewContext(width, height)}
}

func (b *ggDrawBackend) newFont(fonts *fontManager, family FontFamily, size float64) (drawFont, error) {
	key := string(family) + "|" + strconv.FormatFloat(size, 'f', 2, 64)
	if face, ok := b.fonts[key]; ok {
		return face, nil
	}

	ttf, err := fonts.parsedFont()
	if err != nil {
		return nil, err
	}

	face, err := opentype.NewFace(ttf, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingNone,
	})
	if err != nil {
		return nil, err
	}

	metrics := face.Metrics()
	out := &ggDrawFont{
		face:      face,
		measureDC: b.measureDC,
		ascent:    fixedToFloat(metrics.Ascent),
		descent:   fixedToFloat(metrics.Descent),
	}
	b.fonts[key] = out
	return out, nil
}

type ggDrawFont struct {
	face      font.Face
	measureDC *gg.Context
	ascent    float64
	descent   float64
}

func (f *ggDrawFont) metrics() (float64, float64) {
	return f.ascent, f.descent
}

func (f *ggDrawFont) measure(text string) float64 {
	f.measureDC.SetFontFace(f.face)
	width, _ := f.measureDC.MeasureString(text)
	return width
}

type ggDrawCanvas struct {
	dc *gg.Context
}

func (c *ggDrawCanvas) Clear(clr color.Color) {
	c.dc.SetColor(clr)
	c.dc.Clear()
}

func (c *ggDrawCanvas) FillRoundedRect(x, y, w, h, radius float64, clr color.Color) {
	c.dc.SetColor(clr)
	c.dc.DrawRoundedRectangle(x, y, w, h, radius)
	c.dc.Fill()
}

func (c *ggDrawCanvas) StrokeLine(x1, y1, x2, y2, width float64, clr color.Color) {
	c.dc.SetColor(clr)
	c.dc.SetLineWidth(width)
	c.dc.DrawLine(x1, y1, x2, y2)
	c.dc.Stroke()
}

func (c *ggDrawCanvas) FillCircle(x, y, radius float64, clr color.Color) {
	c.dc.SetColor(clr)
	c.dc.DrawCircle(x, y, radius)
	c.dc.Fill()
}

func (c *ggDrawCanvas) DrawText(text string, x, y float64, face drawFont, clr color.Color, fauxItalic bool) {
	fontFace, ok := face.(*ggDrawFont)
	if !ok {
		panic(fmt.Sprintf("gmd: gg canvas received %T font", face))
	}

	c.dc.SetFontFace(fontFace.face)
	c.dc.SetColor(clr)
	if fauxItalic {
		c.dc.Push()
		c.dc.ShearAbout(-0.18, 0, x, y)
	}
	c.dc.DrawString(text, x, y)
	if fauxItalic {
		c.dc.Pop()
	}
}

func (c *ggDrawCanvas) DrawImage(img image.Image, x, y int) {
	c.dc.DrawImage(img, x, y)
}

func (c *ggDrawCanvas) Image() image.Image {
	return c.dc.Image()
}
