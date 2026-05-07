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
	FontEmoji      FontFamily = "emoji"
)

// Options 是渲染器初始化参数。
type Options struct {
	Theme          Theme
	ThemeName      ThemeName
	Width          int
	BaseDir        string // 相对资源路径的基准目录，例如 Markdown 文件所在目录。
	Font           []byte // 正文字体；支持 TTF/OTF/TTC/OTC，不传则使用 Go 内置默认字体。
	FontIndex      int    // 正文字体集合索引；单字体文件固定为 0。
	MonoFont       []byte // 代码字体；支持 TTF/OTF/TTC/OTC，不传则使用 Go 内置等宽字体。
	MonoFontIndex  int    // 代码字体集合索引；单字体文件固定为 0。
	EmojiFont      []byte // Emoji 字体；支持 TTF/OTF/TTC/OTC，不传则回退到正文字体。
	EmojiFontIndex int    // Emoji 字体集合索引；单字体文件固定为 0。
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
	drawContext *gg.Context
}

var (
	MeasureMemoSize = 1024
	FaceCacheSize   = 32
)

// New 创建渲染器实例。
func New(opts Options) (*Renderer, error) {
	theme, err := resolveTheme(opts)
	if err != nil {
		return nil, err
	}

	fonts, err := newFontManager(
		opts.Font,
		opts.FontIndex,
		opts.MonoFont,
		opts.MonoFontIndex,
		opts.EmojiFont,
		opts.EmojiFontIndex,
	)
	if err != nil {
		return nil, err
	}

	return &Renderer{
		theme:       theme,
		baseDir:     normalizeBaseDir(opts.BaseDir),
		md:          goldmark.New(),
		fonts:       fonts,
		measureDC:   gg.NewContext(8, 8),
		measureMemo: make(map[string]float64, MeasureMemoSize),
	}, nil
}

func resolveTheme(opts Options) (Theme, error) {
	if opts.Theme.Width > 0 {
		return opts.Theme, nil
	}

	return ThemeByName(opts.ThemeName, opts.Width), nil
}

func normalizeBaseDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	return filepath.Clean(dir)
}

// SetBaseDir 设置相对资源路径的基准目录，例如 Markdown 文件所在目录。
func (r *Renderer) SetBaseDir(dir string) {
	if r == nil {
		return
	}
	r.baseDir = normalizeBaseDir(dir)
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

// Clear 清理上次绘制遗留的缓存，释放可重建的内存占用。
//
// Clear 不会销毁渲染器配置；调用后仍可继续复用同一个 Renderer 再次渲染。
// 它主要用于主动释放文本测量缓存、字体 face 缓存和临时路径上下文。
func (r *Renderer) Clear() {
	if r == nil {
		return
	}

	r.measureDC = gg.NewContext(8, 8)
	r.drawContext = nil
	clear(r.measureMemo)
	if r.fonts != nil {
		r.fonts.clear()
	}
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
	regularData  []byte
	regularIndex int
	monoData     []byte
	monoIndex    int
	emojiData    []byte
	emojiIndex   int
	customMono   bool
	regularFont  *opentype.Font
	monoFont     *opentype.Font
	emojiFont    *opentype.Font
	faces        map[string]font.Face
}

func newFontManager(
	customFont []byte,
	customFontIndex int,
	customMonoFont []byte,
	customMonoFontIndex int,
	customEmojiFont []byte,
	customEmojiFontIndex int,
) (*fontManager, error) {
	regularData := customFont
	if len(regularData) == 0 {
		regularData = goregular.TTF
	}
	customMono := len(customMonoFont) > 0
	monoData := customMonoFont
	if len(monoData) == 0 {
		monoData = gomono.TTF
	}
	emojiData := customEmojiFont
	if len(emojiData) == 0 {
		emojiData = regularData
		customEmojiFontIndex = customFontIndex
	}

	return &fontManager{
		regularData:  regularData,
		regularIndex: customFontIndex,
		monoData:     monoData,
		monoIndex:    customMonoFontIndex,
		emojiData:    emojiData,
		emojiIndex:   customEmojiFontIndex,
		customMono:   customMono,
		faces:        make(map[string]font.Face, FaceCacheSize),
	}, nil
}

func (m *fontManager) clear() {
	if m == nil {
		return
	}

	for _, face := range m.faces {
		face.Close()
	}

	m.regularFont = nil
	m.monoFont = nil
	m.emojiFont = nil
	clear(m.faces)
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
	data := m.regularData
	index := m.regularIndex
	label := "font"
	cached := &m.regularFont

	if family == FontMono {
		data = m.monoData
		index = m.monoIndex
		label = "mono font"
		cached = &m.monoFont
	} else if family == FontEmoji {
		data = m.emojiData
		index = m.emojiIndex
		label = "emoji font"
		cached = &m.emojiFont
	}

	if *cached != nil {
		return *cached, nil
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%s data is empty", label)
	}
	if index < 0 {
		return nil, fmt.Errorf("%s index must be >= 0, got %d", label, index)
	}

	collection, err := opentype.ParseCollection(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s collection: %w", label, err)
	}

	if index >= collection.NumFonts() {
		return nil, fmt.Errorf(
			"%s index %d out of range: collection has %d font(s)",
			label,
			index,
			collection.NumFonts(),
		)
	}

	font, err := collection.Font(index)
	if err != nil {
		return nil, fmt.Errorf("load %s #%d: %w", label, index, err)
	}

	*cached = font
	return *cached, nil
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

func isRegionalIndicator(r rune) bool {
	return r >= 0x1F1E6 && r <= 0x1F1FF
}

func isKeycapBase(r rune) bool {
	return (r >= '0' && r <= '9') || r == '#' || r == '*'
}

func isEmojiVariationSelector(r rune) bool {
	return r == 0xFE0E || r == 0xFE0F
}

func isEmojiJoiner(r rune) bool {
	return r == 0x200D
}

func isEmojiKeycapMark(r rune) bool {
	return r == 0x20E3
}

func isEmojiRune(r rune) bool {
	switch {
	case r >= 0x1F000 && r <= 0x1FAFF:
		return true
	case r >= 0x2600 && r <= 0x27BF:
		return true
	case r >= 0x2300 && r <= 0x23FF:
		return true
	case r >= 0x2B00 && r <= 0x2BFF:
		return true
	}

	switch r {
	case 0x00A9, 0x00AE, 0x203C, 0x2049, 0x2122, 0x2139, 0x3030, 0x303D, 0x3297, 0x3299:
		return true
	default:
		return false
	}
}

func isEmojiModifier(r rune) bool {
	return r >= 0x1F3FB && r <= 0x1F3FF
}

func isEmojiStart(r rune) bool {
	return isRegionalIndicator(r) || isKeycapBase(r) || isEmojiRune(r)
}

func consumeEmojiCluster(runes []rune, start int) int {
	if start >= len(runes) {
		return start
	}

	i := start
	if isKeycapBase(runes[i]) {
		i++
		if i < len(runes) && isEmojiVariationSelector(runes[i]) {
			i++
		}
		if i < len(runes) && isEmojiKeycapMark(runes[i]) {
			return i + 1
		}
		return start
	}

	if isRegionalIndicator(runes[i]) {
		i++
		if i < len(runes) && isRegionalIndicator(runes[i]) {
			return i + 1
		}
		return i
	}

	if !isEmojiRune(runes[i]) {
		return start
	}

	i++
	for i < len(runes) {
		switch {
		case isEmojiVariationSelector(runes[i]), isEmojiModifier(runes[i]):
			i++
		case isEmojiJoiner(runes[i]):
			i++
			if i >= len(runes) {
				return i
			}
			i++
			for i < len(runes) && (isEmojiVariationSelector(runes[i]) || isEmojiModifier(runes[i])) {
				i++
			}
		default:
			return i
		}
	}

	return i
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

	runes := []rune(text)
	for i := 0; i < len(runes); {
		rn := runes[i]
		switch {
		case unicode.IsSpace(rn):
			flushWord()
			pendingSpace = true
			i++
		case isEmojiStart(rn):
			end := consumeEmojiCluster(runes, i)
			if end > i {
				flushWord()
				flushSpace()
				tokens = append(tokens, string(runes[i:end]))
				i = end
				continue
			}
			flushSpace()
			word = append(word, rn)
			i++
		case isCJK(rn):
			flushWord()
			flushSpace()
			tokens = append(tokens, string(rn))
			i++
		default:
			flushSpace()
			word = append(word, rn)
			i++
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

func splitCodeTokens(text string) []string {
	if text == "" {
		return nil
	}

	var tokens []string
	runes := []rune(text)
	start := 0
	i := 0

	flush := func(end int) {
		if end > start {
			tokens = append(tokens, string(runes[start:end]))
		}
		start = end
	}

	for i < len(runes) {
		if unicode.IsSpace(runes[i]) {
			flush(i)
			j := i + 1
			for j < len(runes) && unicode.IsSpace(runes[j]) {
				j++
			}
			tokens = append(tokens, string(runes[i:j]))
			i = j
			start = j
			continue
		}

		if isEmojiStart(runes[i]) {
			end := consumeEmojiCluster(runes, i)
			if end > i {
				flush(i)
				tokens = append(tokens, string(runes[i:end]))
				i = end
				start = end
				continue
			}
		}

		i++
	}

	flush(len(runes))
	return tokens
}

func hasEmojiCluster(text string) bool {
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		if !isEmojiStart(runes[i]) {
			continue
		}
		if end := consumeEmojiCluster(runes, i); end > i {
			return true
		}
	}
	return false
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
