package gmd

import (
	"fmt"
	"image"
	"image/color"
	"strings"
)

// DrawingLibrary 表示底层绘图库实现。
type DrawingLibrary string

const (
	DrawingLibraryGG       DrawingLibrary = "gg"
	DrawingLibraryTinySkia DrawingLibrary = "tinyskia"
)

// DrawingLibraryNames 返回所有可用绘图库名。
func DrawingLibraryNames() []string {
	return []string{string(DrawingLibraryGG), string(DrawingLibraryTinySkia)}
}

// ParseDrawingLibrary 把外部输入规范化成受支持的绘图库名。
func ParseDrawingLibrary(name string) (DrawingLibrary, error) {
	normalized := DrawingLibrary(strings.ToLower(strings.TrimSpace(name)))
	if normalized == "" {
		return DrawingLibraryGG, nil
	}

	switch normalized {
	case DrawingLibraryGG, DrawingLibraryTinySkia:
		return normalized, nil
	default:
		return "", fmt.Errorf("unknown drawing library %q, supported libraries: %s", name, strings.Join(DrawingLibraryNames(), ", "))
	}
}

type drawBackend interface {
	newCanvas(width, height int) drawCanvas
	newFont(fonts *fontManager, family FontFamily, size float64) (drawFont, error)
}

type drawCanvas interface {
	Clear(c color.Color)
	FillRoundedRect(x, y, w, h, radius float64, c color.Color)
	StrokeLine(x1, y1, x2, y2, width float64, c color.Color)
	FillCircle(x, y, radius float64, c color.Color)
	DrawText(text string, x, y float64, font drawFont, c color.Color, fauxItalic bool)
	DrawImage(img image.Image, x, y int)
	Image() image.Image
}

type drawFont interface {
	metrics() (ascent, descent float64)
	measure(text string) float64
}

func newDrawBackend(lib DrawingLibrary) (drawBackend, error) {
	name, err := ParseDrawingLibrary(string(lib))
	if err != nil {
		return nil, err
	}

	switch name {
	case DrawingLibraryTinySkia:
		return newTinySkiaDrawBackend(), nil
	default:
		return newGGDrawBackend(), nil
	}
}

// draw 根据布局结果真正输出位图。
func (r *Renderer) draw(doc *layoutDocument) (image.Image, error) {
	dc := r.drawer.newCanvas(doc.Width, doc.Height)
	dc.Clear(r.theme.Background)

	for _, block := range doc.Blocks {
		if err := r.drawBlock(dc, block); err != nil {
			return nil, err
		}
	}

	return dc.Image(), nil
}

// drawBlock 按块级类型分派到不同绘制逻辑。
func (r *Renderer) drawBlock(dc drawCanvas, block layoutBlock) error {
	switch block.Kind {
	case blockParagraph, blockHeading:
		return r.drawLines(dc, block.Lines)
	case blockCode:
		dc.FillRoundedRect(block.X, block.Y, block.Width, block.Height, r.theme.Radius, r.theme.CodeFill)
		return r.drawLines(dc, block.Lines)
	case blockRule:
		y := block.Y + block.Height/2
		dc.StrokeLine(block.X, y, block.X+block.Width, y, 1.5, r.theme.Rule)
		return nil
	case blockImage:
		if block.Image == nil {
			return nil
		}
		return r.drawImageBlock(dc, block)
	case blockBlockquote:
		dc.FillRoundedRect(block.X, block.Y, block.Width, block.Height, r.theme.Radius, r.theme.QuoteFill)
		dc.FillRoundedRect(block.X, block.Y, r.theme.QuoteBarWidth, block.Height, r.theme.QuoteBarWidth/2, r.theme.QuoteBar)
		for _, child := range block.Children {
			if err := r.drawBlock(dc, child); err != nil {
				return err
			}
		}
	case blockList:
		for _, item := range block.Items {
			if item.UseBullet {
				cx := item.MarkerX + 7
				cy := item.MarkerBaseline - item.MarkerStyle.Ascent*0.35
				dc.FillCircle(cx, cy, 3.2, item.MarkerStyle.Color)
			} else {
				dc.DrawText(item.Marker, item.MarkerX, item.MarkerBaseline, item.MarkerStyle.Face, item.MarkerStyle.Color, false)
			}
			for _, child := range item.Blocks {
				if err := r.drawBlock(dc, child); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (r *Renderer) drawImageBlock(dc drawCanvas, block layoutBlock) error {
	bounds := block.Image.Bounds()
	srcWidth := float64(bounds.Dx())
	srcHeight := float64(bounds.Dy())
	if nearlyZero(srcWidth) || nearlyZero(srcHeight) || nearlyZero(block.Width) || nearlyZero(block.Height) {
		return nil
	}

	if sameSize(srcWidth, block.Width) && sameSize(srcHeight, block.Height) {
		dc.DrawImage(block.Image, int(block.X), int(block.Y))
		return nil
	}

	scaled := scaleImage(block.Image, int(block.Width+0.5), int(block.Height+0.5))
	dc.DrawImage(scaled, int(block.X), int(block.Y))
	return nil
}

// drawLines 负责真正输出文字与行内代码背景。
func (r *Renderer) drawLines(dc drawCanvas, lines []layoutLine) error {
	for _, line := range lines {
		for _, frag := range line.Fragments {
			if frag.Style.InlineCode {
				paddingY := r.theme.InlineCodePaddingY
				boxX := frag.X
				boxY := line.Baseline - frag.Style.Ascent - paddingY
				boxW := frag.Width
				boxH := frag.Style.Ascent + frag.Style.Descent + paddingY*2

				dc.FillRoundedRect(boxX, boxY, boxW, boxH, 6, r.theme.InlineCode)
			}

			textX := frag.X + frag.TextOffsetX
			dc.DrawText(frag.Text, textX, line.Baseline, frag.Style.Face, frag.Style.Color, frag.Style.FauxItalic)
			if frag.Style.FauxBold {
				dc.DrawText(frag.Text, textX+0.8, line.Baseline, frag.Style.Face, frag.Style.Color, frag.Style.FauxItalic)
			}

			if frag.Style.Underline {
				y := line.Baseline + 2
				dc.StrokeLine(textX, y, textX+frag.TextWidth, y, 1.2, frag.Style.Color)
			}
		}
	}

	return nil
}
