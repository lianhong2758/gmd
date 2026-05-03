package gmd

import (
	"fmt"
	"image"
	"strings"
)

type layoutDocument struct {
	Width  int
	Height int
	Blocks []layoutBlock
}

type layoutBlock struct {
	Kind     blockKind
	X        float64
	Y        float64
	Width    float64
	Height   float64
	Image    image.Image
	Lines    []layoutLine
	Items    []layoutListItem
	Children []layoutBlock
}

type layoutListItem struct {
	Marker         string
	UseBullet      bool
	MarkerStyle    resolvedTextStyle
	MarkerX        float64
	MarkerBaseline float64
	Height         float64
	Blocks         []layoutBlock
}

type layoutLine struct {
	Height    float64
	Baseline  float64
	Fragments []layoutFragment
}

type layoutFragment struct {
	Text        string
	X           float64
	Width       float64
	TextWidth   float64
	TextOffsetX float64
	Style       resolvedTextStyle
}

type rawLine struct {
	Width     float64
	Height    float64
	Ascent    float64
	Descent   float64
	Fragments []layoutFragment
}

type spanToken struct {
	Text       string
	Style      resolvedTextStyle
	Space      bool
	ForceBreak bool
}

// layoutDocument 根据固定宽度完成整份文档的排版。
// 这一步只计算位置，不真正绘图，便于后续重复输出到不同目标。
func (r *Renderer) layoutDocument(doc *document) (*layoutDocument, error) {
	usableWidth := float64(r.theme.Width) - r.theme.Padding*2
	blocks, bottom, err := r.layoutBlocks(doc.Blocks, r.theme.Padding, r.theme.Padding, usableWidth, 0)
	if err != nil {
		return nil, err
	}

	height := int(bottom + r.theme.Padding)
	if height < 1 {
		height = 1
	}

	return &layoutDocument{
		Width:  r.theme.Width,
		Height: height,
		Blocks: blocks,
	}, nil
}

func (r *Renderer) layoutBlocks(blocks []block, x, y, width float64, quoteDepth int) ([]layoutBlock, float64, error) {
	var out []layoutBlock
	cursor := y

	for idx, b := range blocks {
		var lb layoutBlock
		var err error

		switch b.Kind {
		case blockHeading:
			lb, err = r.layoutTextualBlock(b, x, cursor, width, r.headingStyle(b.Level))
		case blockParagraph:
			lb, err = r.layoutTextualBlock(b, x, cursor, width, r.paragraphStyle(quoteDepth))
		case blockCode:
			lb, err = r.layoutCodeBlock(b, x, cursor, width)
		case blockRule:
			lb = layoutBlock{
				Kind:   blockRule,
				X:      x,
				Y:      cursor,
				Width:  width,
				Height: r.theme.RuleSpacing,
			}
		case blockImage:
			lb, err = r.layoutImageBlock(b, x, cursor, width, quoteDepth)
		case blockBlockquote:
			lb, err = r.layoutQuoteBlock(b, x, cursor, width, quoteDepth)
		case blockList:
			lb, err = r.layoutListBlock(b, x, cursor, width, quoteDepth)
		default:
			continue
		}
		if err != nil {
			return nil, 0, err
		}

		out = append(out, lb)
		cursor += lb.Height
		if idx != len(blocks)-1 {
			cursor += r.blockGap(b.Kind)
		}
	}

	return out, cursor, nil
}

// layoutTextualBlock 用统一的文本布局逻辑处理标题和段落。
func (r *Renderer) layoutTextualBlock(b block, x, y, width float64, base textStyle) (layoutBlock, error) {
	lines, err := r.wrapInlineSpans(b.Inlines, base, width)
	if err != nil {
		return layoutBlock{}, err
	}

	placed, total := placeRawLines(lines, x, y)
	return layoutBlock{
		Kind:   b.Kind,
		X:      x,
		Y:      y,
		Width:  width,
		Height: total,
		Lines:  placed,
	}, nil
}

func (r *Renderer) layoutImageBlock(b block, x, y, width float64, quoteDepth int) (layoutBlock, error) {
	img, err := r.loadImage(b.Image.Source)
	if err != nil {
		fallback := block{
			Kind: blockParagraph,
			Inlines: []inlineSpan{{
				Text: "[图片: " + b.Image.Alt + "]",
			}},
		}
		return r.layoutTextualBlock(fallback, x, y, width, r.paragraphStyle(quoteDepth))
	}

	bounds := img.Bounds()
	drawWidth := float64(bounds.Dx())
	drawHeight := float64(bounds.Dy())
	if drawWidth > width && drawWidth > 0 {
		scale := width / drawWidth
		drawWidth = width
		drawHeight *= scale
	}

	return layoutBlock{
		Kind:   blockImage,
		X:      x,
		Y:      y,
		Width:  drawWidth,
		Height: drawHeight,
		Image:  img,
	}, nil
}

// layoutCodeBlock 专门处理代码块：
// 1. 先可选绘制语言标签
// 2. 再按等宽字体换行
// 3. 最后给整块包一个背景面板
func (r *Renderer) layoutCodeBlock(b block, x, y, width float64) (layoutBlock, error) {
	codeStyle := r.codeStyle()
	resolved, err := r.resolveStyle(codeStyle)
	if err != nil {
		return layoutBlock{}, err
	}

	var raw []rawLine
	if info := strings.TrimSpace(b.Info); info != "" {
		labelStyle := codeStyle
		labelStyle.Size = maxFloat(14, codeStyle.Size*0.72)
		labelStyle.Color = r.theme.InlineCode
		labelResolved, err := r.resolveStyle(labelStyle)
		if err != nil {
			return layoutBlock{}, err
		}

		labelWidth := r.measure(labelResolved, info)
		raw = append(raw, rawLine{
			Width:   labelWidth,
			Height:  labelResolved.LinePx,
			Ascent:  labelResolved.Ascent,
			Descent: labelResolved.Descent,
			Fragments: []layoutFragment{{
				Text:  info,
				Width: labelWidth,
				Style: labelResolved,
			}},
		})
		raw = append(raw, rawLine{Height: 8})
	}

	innerWidth := width - r.theme.CodePaddingX*2
	if innerWidth < 1 {
		innerWidth = 1
	}

	codeLines, err := r.wrapCodeText(b.Text, resolved, innerWidth)
	if err != nil {
		return layoutBlock{}, err
	}
	raw = append(raw, codeLines...)

	placed, total := placeRawLines(raw, x+r.theme.CodePaddingX, y+r.theme.CodePaddingY)
	return layoutBlock{
		Kind:   blockCode,
		X:      x,
		Y:      y,
		Width:  width,
		Height: total + r.theme.CodePaddingY*2,
		Lines:  placed,
	}, nil
}

// layoutQuoteBlock 在子内容外层增加引用背景、左侧竖条和额外内边距。
func (r *Renderer) layoutQuoteBlock(b block, x, y, width float64, quoteDepth int) (layoutBlock, error) {
	innerX := x + r.theme.QuoteBarWidth + r.theme.QuotePaddingX
	innerY := y + r.theme.QuotePaddingY
	innerWidth := width - r.theme.QuoteBarWidth - r.theme.QuotePaddingX*2
	if innerWidth < 1 {
		innerWidth = 1
	}

	children, bottom, err := r.layoutBlocks(b.Blocks, innerX, innerY, innerWidth, quoteDepth+1)
	if err != nil {
		return layoutBlock{}, err
	}

	return layoutBlock{
		Kind:     blockBlockquote,
		X:        x,
		Y:        y,
		Width:    width,
		Height:   (bottom - y) + r.theme.QuotePaddingY,
		Children: children,
	}, nil
}

// layoutListBlock 为列表项预留 marker 区域，再把正文块布局到右侧内容区。
func (r *Renderer) layoutListBlock(b block, x, y, width float64, quoteDepth int) (layoutBlock, error) {
	baseStyle, err := r.resolveStyle(r.paragraphStyle(quoteDepth))
	if err != nil {
		return layoutBlock{}, err
	}

	markerWidth := r.theme.ListIndent
	contentX := x + markerWidth
	contentWidth := width - markerWidth
	if contentWidth < 1 {
		contentWidth = 1
	}

	cursor := y
	var items []layoutListItem
	counter := b.Start
	if counter <= 0 {
		counter = 1
	}

	for idx, item := range b.Items {
		marker := ""
		useBullet := true
		if b.Ordered {
			marker = fmt.Sprintf("%d.", counter)
			counter++
			useBullet = false
		}

		children, bottom, err := r.layoutBlocks(item.Blocks, contentX, cursor, contentWidth, quoteDepth)
		if err != nil {
			return layoutBlock{}, err
		}

		baseline := firstBaseline(children, cursor+baseStyle.Ascent)
		itemHeight := bottom - cursor
		if itemHeight < baseStyle.LinePx {
			itemHeight = baseStyle.LinePx
		}

		items = append(items, layoutListItem{
			Marker:         marker,
			UseBullet:      useBullet,
			MarkerStyle:    baseStyle,
			MarkerX:        x,
			MarkerBaseline: baseline,
			Height:         itemHeight,
			Blocks:         children,
		})

		cursor = bottom
		if idx != len(b.Items)-1 {
			if b.Tight {
				cursor += minFloat(6, r.theme.ListItemGap)
			} else {
				cursor += r.theme.ListItemGap
			}
		}
	}

	return layoutBlock{
		Kind:   blockList,
		X:      x,
		Y:      y,
		Width:  width,
		Height: cursor - y,
		Items:  items,
	}, nil
}

func (r *Renderer) paragraphStyle(quoteDepth int) textStyle {
	colorValue := r.theme.Text
	if quoteDepth > 0 {
		colorValue = r.theme.MutedText
	}

	return textStyle{
		Family:     FontRegular,
		Size:       r.theme.BaseFontSize,
		LineHeight: r.theme.BaseLineHeight,
		Color:      colorValue,
	}
}

func (r *Renderer) headingStyle(level int) textStyle {
	if level < 1 {
		level = 1
	}
	if level > 6 {
		level = 6
	}

	size := r.theme.BaseFontSize * r.theme.HeadingScale[level-1]
	lineHeight := 1.18
	if level >= 4 {
		lineHeight = 1.25
	}

	style := textStyle{
		Family:     FontRegular,
		Size:       size,
		LineHeight: lineHeight,
		Color:      r.theme.Text,
		FauxBold:   true,
	}
	return style
}

func (r *Renderer) codeStyle() textStyle {
	return textStyle{
		Family:     FontMono,
		Size:       r.theme.BaseFontSize * 0.90,
		LineHeight: r.theme.CodeLineHeight,
		Color:      r.theme.CodeText,
	}
}

func (r *Renderer) applyInlineStyle(base textStyle, span inlineSpan) textStyle {
	style := base

	if span.Code {
		return textStyle{
			Family:     FontMono,
			Size:       base.Size * 0.92,
			LineHeight: base.LineHeight,
			Color:      r.theme.Text,
			InlineCode: true,
		}
	}

	style.Family = FontRegular
	style.FauxBold = style.FauxBold || span.Bold
	style.FauxItalic = style.FauxItalic || span.Italic

	if span.Link {
		style.Color = r.theme.Link
		style.Underline = true
	}

	return style
}

// wrapInlineSpans 把行内片段转换成可换行 token，再生成最终行集合。
// 代码片段会保留成单独 token，避免被普通空格分词逻辑破坏。
func (r *Renderer) wrapInlineSpans(spans []inlineSpan, base textStyle, width float64) ([]rawLine, error) {
	var tokens []spanToken

	for _, span := range spans {
		if span.ForceBreak {
			tokens = append(tokens, spanToken{ForceBreak: true})
			continue
		}
		if span.Text == "" {
			continue
		}

		style, err := r.resolveStyle(r.applyInlineStyle(base, span))
		if err != nil {
			return nil, err
		}

		if span.Code {
			tokens = append(tokens, spanToken{
				Text:  span.Text,
				Style: style,
			})
			continue
		}

		for _, token := range splitPlainTokens(span.Text) {
			tokens = append(tokens, spanToken{
				Text:  token,
				Style: style,
				Space: token == " ",
			})
		}
	}

	if len(tokens) == 0 {
		style, err := r.resolveStyle(base)
		if err != nil {
			return nil, err
		}
		return []rawLine{{
			Height:  style.LinePx,
			Ascent:  style.Ascent,
			Descent: style.Descent,
		}}, nil
	}

	return r.wrapTokens(tokens, width)
}

// wrapCodeText 以“保留每一行”的方式处理代码块文本。
// 如果某一行过长，会继续按宽度切分，避免整张图被超长代码撑爆。
func (r *Renderer) wrapCodeText(text string, style resolvedTextStyle, width float64) ([]rawLine, error) {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.TrimSuffix(normalized, "\n")
	lines := strings.Split(normalized, "\n")
	var raw []rawLine

	for _, line := range lines {
		if line == "" {
			raw = append(raw, rawLine{
				Height:  style.LinePx,
				Ascent:  style.Ascent,
				Descent: style.Descent,
			})
			continue
		}

		wrapped, err := r.wrapTokens([]spanToken{{
			Text:  line,
			Style: style,
		}}, width)
		if err != nil {
			return nil, err
		}
		raw = append(raw, wrapped...)
	}

	return raw, nil
}

// wrapTokens 是核心换行算法。
// 它会尽量复用已经度量过的字符串宽度，减少重复 MeasureString 开销。
func (r *Renderer) wrapTokens(tokens []spanToken, width float64) ([]rawLine, error) {
	var lines []rawLine
	var frags []layoutFragment
	lineWidth := 0.0
	lineHeight := 0.0
	lineAscent := 0.0
	lineDescent := 0.0

	flushLine := func(force bool) {
		trimmed, total := trimRightSpaces(frags)
		frags = trimmed
		lineWidth = total

		if len(frags) == 0 && !force {
			lineHeight = 0
			lineAscent = 0
			lineDescent = 0
			return
		}

		lines = append(lines, rawLine{
			Width:     lineWidth,
			Height:    maxFloat(lineHeight, 1),
			Ascent:    lineAscent,
			Descent:   lineDescent,
			Fragments: frags,
		})

		frags = nil
		lineWidth = 0
		lineHeight = 0
		lineAscent = 0
		lineDescent = 0
	}

	addFragment := func(text string, style resolvedTextStyle) {
		if text == "" {
			return
		}

		textWidth := r.measure(style, text)
		widthValue := textWidth
		textOffsetX := 0.0
		if style.InlineCode {
			textOffsetX = r.theme.InlineCodePaddingX
			widthValue += r.theme.InlineCodePaddingX * 2
		}

		if len(frags) > 0 {
			last := &frags[len(frags)-1]
			if last.Style.MeasureID == style.MeasureID &&
				last.Style.Underline == style.Underline &&
				last.Style.InlineCode == style.InlineCode &&
				last.Style.Color == style.Color {
				last.Text += text
				last.Width += widthValue
				last.TextWidth += textWidth
			} else {
				frags = append(frags, layoutFragment{
					Text:        text,
					Width:       widthValue,
					TextWidth:   textWidth,
					TextOffsetX: textOffsetX,
					Style:       style,
				})
			}
		} else {
			frags = append(frags, layoutFragment{
				Text:        text,
				Width:       widthValue,
				TextWidth:   textWidth,
				TextOffsetX: textOffsetX,
				Style:       style,
			})
		}

		lineWidth += widthValue
		lineHeight = maxFloat(lineHeight, style.LinePx)
		lineAscent = maxFloat(lineAscent, style.Ascent)
		lineDescent = maxFloat(lineDescent, style.Descent)
	}

	appendToken := func(token spanToken) {
		if token.Text == "" {
			return
		}

		tokenWidth := r.measure(token.Style, token.Text)
		if token.Style.InlineCode {
			tokenWidth += r.theme.InlineCodePaddingX * 2
		}
		if nearlyZero(lineWidth) && token.Space {
			return
		}

		if lineWidth+tokenWidth <= width {
			addFragment(token.Text, token.Style)
			return
		}

		if nearlyZero(lineWidth) && tokenWidth <= width {
			addFragment(token.Text, token.Style)
			return
		}

		if !nearlyZero(lineWidth) {
			flushLine(false)
		}

		if token.Space {
			return
		}

		remain := token.Text
		for remain != "" {
			head, tail := chunkRunesByWidth(remain, func(part string) bool {
				return r.measure(token.Style, part) <= width
			})
			addFragment(head, token.Style)
			remain = tail
			if remain != "" {
				flushLine(false)
			}
		}
	}

	for _, token := range tokens {
		if token.ForceBreak {
			flushLine(true)
			continue
		}
		appendToken(token)
	}

	flushLine(false)
	if len(lines) == 0 {
		lines = append(lines, rawLine{Height: 1})
	}
	return lines, nil
}

// placeRawLines 把“相对行信息”转换成带绝对坐标的布局结果。
func placeRawLines(raw []rawLine, x, y float64) ([]layoutLine, float64) {
	var placed []layoutLine
	cursor := y

	for _, line := range raw {
		baseline := cursor + line.Ascent
		offsetX := 0.0
		var frags []layoutFragment

		for _, frag := range line.Fragments {
			frag.X = x + offsetX
			frags = append(frags, frag)
			offsetX += frag.Width
		}

		placed = append(placed, layoutLine{
			Height:    line.Height,
			Baseline:  baseline,
			Fragments: frags,
		})
		cursor += line.Height
	}

	return placed, cursor - y
}

func firstBaseline(blocks []layoutBlock, fallback float64) float64 {
	for _, block := range blocks {
		for _, line := range block.Lines {
			return line.Baseline
		}
		if v := firstBaseline(block.Children, 0); v > 0 {
			return v
		}
		for _, item := range block.Items {
			if v := firstBaseline(item.Blocks, 0); v > 0 {
				return v
			}
		}
	}
	return fallback
}

func (r *Renderer) blockGap(kind blockKind) float64 {
	switch kind {
	case blockParagraph:
		return r.theme.ParagraphGap
	case blockRule:
		return r.theme.RuleSpacing
	default:
		return r.theme.BlockGap
	}
}
