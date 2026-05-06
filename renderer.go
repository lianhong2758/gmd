package gmd

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/FloatTech/gg"
	"github.com/yuin/goldmark"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// FontFamily 表示文本样式标签。
// 常规文本使用 Options.Font，代码优先使用 Options.MonoFont。
type FontFamily string

const (
	FontRegular    FontFamily = "regular"
	FontBold       FontFamily = "bold"
	FontItalic     FontFamily = "italic"
	FontBoldItalic FontFamily = "bolditalic"
	FontMono       FontFamily = "mono"
)

// Options 是渲染器初始化参数。
type Options struct {
	Theme     Theme
	ThemeName ThemeName
	Width     int
	Font      []byte // 正文字体；不传则使用 Go 内置默认字体。
	MonoFont  []byte // 代码字体；不传则使用 Go 内置等宽字体。
}

// Renderer 负责 Markdown 的解析、布局与绘制。
//
// 为了性能更稳定，这里采用“三段式”流程：
// 1. Markdown AST 转内部结构
// 2. 根据目标宽度做布局
// 3. 一次性绘制到图片
type Renderer struct {
	theme       Theme
	baseDir     string
	md          goldmark.Markdown
	fonts       *fontManager
	measureDC   *gg.Context
	measureMemo map[string]float64
}

// New 创建渲染器实例。
func New(opts Options) (*Renderer, error) {
	theme, err := resolveTheme(opts)
	if err != nil {
		return nil, err
	}

	fonts, err := newFontManager(opts.Font, opts.MonoFont)
	if err != nil {
		return nil, err
	}

	return &Renderer{
		theme:       theme,
		md:          goldmark.New(),
		fonts:       fonts,
		measureDC:   gg.NewContext(8, 8),
		measureMemo: make(map[string]float64, 1024),
	}, nil
}

func resolveTheme(opts Options) (Theme, error) {
	if opts.Theme.Width > 0 {
		return opts.Theme, nil
	}

	name, err := ParseThemeName(string(opts.ThemeName))
	if err != nil {
		return Theme{}, err
	}

	return ThemeByName(name, opts.Width), nil
}

// Render 把 Markdown 渲染为内存中的图片对象。
func (r *Renderer) Render(markdown []byte) (image.Image, error) {
	doc, err := r.parse(markdown)
	if err != nil {
		return nil, err
	}

	layout, err := r.layoutDocument(doc)
	if err != nil {
		return nil, err
	}

	return r.draw(layout)
}

// RenderToFile 直接把 Markdown 输出为 PNG 文件。
func (r *Renderer) RenderToFile(markdown []byte, outputPath string) error {
	img, err := r.Render(markdown)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}

type fontManager struct {
	regularData []byte
	monoData    []byte
	customMono  bool
	regularFont *opentype.Font
	monoFont    *opentype.Font
	faces       map[string]font.Face
}

func newFontManager(customFont, customMonoFont []byte) (*fontManager, error) {
	regularData := customFont
	if len(regularData) == 0 {
		regularData = goregular.TTF
	}
	customMono := len(customMonoFont) > 0
	monoData := customMonoFont
	if len(monoData) == 0 {
		monoData = gomono.TTF
	}

	return &fontManager{
		regularData: regularData,
		monoData:    monoData,
		customMono:  customMono,
		faces:       make(map[string]font.Face, 32),
	}, nil
}

func (m *fontManager) face(family FontFamily, size float64) (font.Face, error) {
	key := string(family) + "|" + strconv.FormatFloat(size, 'f', 2, 64)
	if face, ok := m.faces[key]; ok {
		return face, nil
	}

	ttf, err := m.parsedFont(family)
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

	m.faces[key] = face
	return face, nil
}

func (m *fontManager) parsedFont(family FontFamily) (*opentype.Font, error) {
	if family == FontMono {
		if m.monoFont != nil {
			return m.monoFont, nil
		}
		if len(m.monoData) == 0 {
			return nil, fmt.Errorf("mono font data is empty")
		}
		ttf, err := opentype.Parse(m.monoData)
		if err != nil {
			return nil, fmt.Errorf("parse mono font: %w", err)
		}
		m.monoFont = ttf
		return m.monoFont, nil
	}

	if m.regularFont != nil {
		return m.regularFont, nil
	}
	if len(m.regularData) == 0 {
		return nil, fmt.Errorf("font data is empty")
	}

	ttf, err := opentype.Parse(m.regularData)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}

	m.regularFont = ttf
	return m.regularFont, nil
}

type textStyle struct {
	Family     FontFamily
	Size       float64
	LineHeight float64
	Color      color.Color
	Underline  bool
	InlineCode bool
	FauxBold   bool
	FauxItalic bool
}

type resolvedTextStyle struct {
	textStyle
	Face      font.Face
	Ascent    float64
	Descent   float64
	LinePx    float64
	MeasureID string
}

func (r *Renderer) resolveStyle(style textStyle) (resolvedTextStyle, error) {
	face, err := r.fonts.face(style.Family, style.Size)
	if err != nil {
		return resolvedTextStyle{}, err
	}

	metrics := face.Metrics()
	return resolvedTextStyle{
		textStyle: style,
		Face:      face,
		Ascent:    fixedToFloat(metrics.Ascent),
		Descent:   fixedToFloat(metrics.Descent),
		LinePx:    style.Size * style.LineHeight,
		MeasureID: string(style.Family) + "|" + strconv.FormatFloat(style.Size, 'f', 2, 64) +
			"|b=" + strconv.FormatBool(style.FauxBold) +
			"|i=" + strconv.FormatBool(style.FauxItalic),
	}, nil
}

func (r *Renderer) measure(style resolvedTextStyle, text string) float64 {
	if text == "" {
		return 0
	}

	key := style.MeasureID + "|" + text
	if width, ok := r.measureMemo[key]; ok {
		return width
	}

	r.measureDC.SetFontFace(style.Face)
	width, _ := r.measureDC.MeasureString(text)
	r.measureMemo[key] = width
	return width
}

func rgb(r, g, b uint8) color.RGBA {
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func fixedToFloat(v fixed.Int26_6) float64 {
	return float64(v) / 64
}

func maxFloat(values ...float64) float64 {
	best := values[0]
	for _, v := range values[1:] {
		if v > best {
			best = v
		}
	}
	return best
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func isCJK(r rune) bool {
	return (r >= 0x2E80 && r <= 0x9FFF) || (r >= 0xF900 && r <= 0xFAFF)
}

func trimRightSpaces(frags []layoutFragment) ([]layoutFragment, float64) {
	if len(frags) == 0 {
		return frags, 0
	}

	total := 0.0
	for _, frag := range frags {
		total += frag.Width
	}

	for len(frags) > 0 {
		last := frags[len(frags)-1]
		if strings.TrimSpace(last.Text) != "" {
			break
		}
		total -= last.Width
		frags = frags[:len(frags)-1]
	}

	return frags, total
}

func splitPlainTokens(text string) []string {
	var tokens []string
	var word []rune
	pendingSpace := false

	flushWord := func() {
		if len(word) == 0 {
			return
		}
		tokens = append(tokens, string(word))
		word = word[:0]
	}

	flushSpace := func() {
		if pendingSpace {
			tokens = append(tokens, " ")
			pendingSpace = false
		}
	}

	for _, rn := range text {
		switch {
		case unicode.IsSpace(rn):
			flushWord()
			pendingSpace = true
		case isCJK(rn):
			flushWord()
			flushSpace()
			tokens = append(tokens, string(rn))
		default:
			flushSpace()
			word = append(word, rn)
		}
	}

	flushWord()
	return tokens
}

func expandCodeTabs(text string, tabWidth int) string {
	if tabWidth <= 0 || !strings.ContainsRune(text, '\t') {
		return text
	}

	var sb strings.Builder
	column := 0
	for _, rn := range text {
		if rn == '\t' {
			spaces := tabWidth - column%tabWidth
			sb.WriteString(strings.Repeat(" ", spaces))
			column += spaces
			continue
		}

		sb.WriteRune(rn)
		column += codeColumnWidth(rn)
	}
	return sb.String()
}

func hasNonASCII(text string) bool {
	for _, rn := range text {
		if rn > unicode.MaxASCII {
			return true
		}
	}
	return false
}

func (r *Renderer) shouldFallbackCodeText(text string) bool {
	return !r.fonts.customMono && hasNonASCII(text)
}

func codeColumnWidth(rn rune) int {
	if isCJK(rn) {
		return 2
	}
	return 1
}

func chunkRunesByWidth(text string, fit func(string) bool) (head, tail string) {
	runes := []rune(text)
	if len(runes) == 0 {
		return "", ""
	}

	lo, hi := 1, len(runes)
	best := 1
	for lo <= hi {
		mid := (lo + hi) / 2
		part := string(runes[:mid])
		if fit(part) {
			best = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}

	return string(runes[:best]), string(runes[best:])
}

func nearlyZero(v float64) bool {
	return math.Abs(v) < 0.0001
}
