package gmd

import (
	"fmt"
	"image"
	"strings"

	"github.com/FloatTech/gg"
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
	BulletFilled   bool
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

const codeTabWidth = 4

// layoutDocument 根据固定宽度完成整份文档的排版。
// 这一步只计算位置，不真正绘图，便于后续重复输出到不同目标。
func (r *Renderer) layoutDocument(doc *document) (*layoutDocument, error) {
	usableWidth := float64(r.theme.Width) - r.theme.Padding*2
	blocks, bottom, err := r.layoutBlocks(doc.Blocks, r.theme.Padding, r.theme.Padding, usableWidth, 0, 0)
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

func (r *Renderer) layoutBlocks(blocks []block, x, y, width float64, quoteDepth, listDepth int) ([]layoutBlock, float64, error) {
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
			lb, err = r.layoutQuoteBlock(b, x, cursor, width, quoteDepth, listDepth)
		case blockList:
			lb, err = r.layoutListBlock(b, x, cursor, width, quoteDepth, listDepth)
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
		labelStyle.Color = r.theme.CodeLabel
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
func (r *Renderer) layoutQuoteBlock(b block, x, y, width float64, quoteDepth, listDepth int) (layoutBlock, error) {
	innerX := x + r.theme.QuoteBarWidth + r.theme.QuotePaddingX
	innerY := y + r.theme.QuotePaddingY
	innerWidth := width - r.theme.QuoteBarWidth - r.theme.QuotePaddingX*2
	if innerWidth < 1 {
		innerWidth = 1
	}

	children, bottom, err := r.layoutBlocks(b.Blocks, innerX, innerY, innerWidth, quoteDepth+1, listDepth)
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
func (r *Renderer) layoutListBlock(b block, x, y, width float64, quoteDepth, listDepth int) (layoutBlock, error) {
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

		children, bottom, err := r.layoutBlocks(item.Blocks, contentX, cursor, contentWidth, quoteDepth, listDepth+1)
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
			BulletFilled:   listDepth == 0,
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
			Color:      r.theme.InlineText,
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

func (r *Renderer) resolveStyleFamily(base resolvedTextStyle, family FontFamily) (resolvedTextStyle, error) {
	if base.Family == family {
		return base, nil
	}

	alt := base.textStyle
	alt.Family = family
	return r.resolveStyle(alt)
}

func (r *Renderer) resolvedInlineTokenStyle(base resolvedTextStyle, token string, fallbackRegular bool) (resolvedTextStyle, error) {
	if token == "" {
		return base, nil
	}

	runes := []rune(token)
	if len(runes) > 0 && isEmojiStart(runes[0]) {
		if end := consumeEmojiCluster(runes, 0); end == len(runes) {
			return r.resolveStyleFamily(base, FontEmoji)
		}
	}

	if fallbackRegular && hasNonASCII(token) {
		return r.resolveStyleFamily(base, FontRegular)
	}

	return base, nil
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

		textStyle := r.applyInlineStyle(base, span)
		style, err := r.resolveStyle(textStyle)
		if err != nil {
			return nil, err
		}

		if span.Code {
			fallbackRegular := r.shouldFallbackCodeText(span.Text)
			if !hasEmojiCluster(span.Text) {
				tokenStyle := style
				if fallbackRegular {
					tokenStyle, err = r.resolveStyleFamily(style, FontRegular)
					if err != nil {
						return nil, err
					}
				}
				tokens = append(tokens, spanToken{
					Text:  span.Text,
					Style: tokenStyle,
				})
				continue
			}

			for _, token := range splitCodeTokens(span.Text) {
				tokenStyle, err := r.resolvedInlineTokenStyle(style, token, fallbackRegular)
				if err != nil {
					return nil, err
				}
				tokens = append(tokens, spanToken{
					Text:  token,
					Style: tokenStyle,
				})
			}
			continue
		}

		// goldmark 在相邻 inline 元素之间可能给出“仅包含空白”的独立文本节点。
		// 这里直接保留这个空白，而不是依赖普通分词逻辑去推断。
		if strings.TrimSpace(span.Text) == "" {
			tokens = append(tokens, spanToken{
				Text:  " ",
				Style: style,
				Space: true,
			})
			continue
		}

		for _, token := range splitPlainTokens(span.Text) {
			tokenStyle, err := r.resolvedInlineTokenStyle(style, token, false)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, spanToken{
				Text:  token,
				Style: tokenStyle,
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
// 代码块的空白宽度按固定代码列计算，避免 tab/空格被当前字体绘制成不稳定缩进。
// 如果某一行过长，会继续按宽度切分，避免整张图被超长代码撑爆。
func (r *Renderer) wrapCodeText(text string, style resolvedTextStyle, width float64) ([]rawLine, error) {
	fallback := style
	useFallback := r.shouldFallbackCodeText(text)
	if useFallback {
		fallbackStyle := style.textStyle
		fallbackStyle.Family = FontRegular
		var err error
		fallback, err = r.resolveStyle(fallbackStyle)
		if err != nil {
			return nil, err
		}
	}

	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.TrimSuffix(normalized, "\n")
	var raw []rawLine

	for {
		line, rest, ok := strings.Cut(normalized, "\n")
		raw = append(raw, r.wrapCodeLine(expandCodeTabs(line, codeTabWidth), style, fallback, useFallback, width)...)
		if !ok {
			break
		}
		normalized = rest
	}

	return raw, nil
}

func (r *Renderer) wrapCodeLine(line string, style, fallback resolvedTextStyle, useFallback bool, width float64) []rawLine {
	if line == "" {
		return []rawLine{r.emptyCodeLine(style)}
	}

	cellWidth := r.measure(style, " ")
	var lines []rawLine
	var frags []layoutFragment
	lineWidth := 0.0

	emojiStyle := resolvedTextStyle{}
	emojiStyleReady := false

	ensureEmojiStyle := func() (resolvedTextStyle, error) {
		if emojiStyleReady {
			return emojiStyle, nil
		}
		alt, err := r.resolveStyleFamily(style, FontEmoji)
		if err != nil {
			return resolvedTextStyle{}, err
		}
		emojiStyle = alt
		emojiStyleReady = true
		return emojiStyle, nil
	}

	flushLine := func(force bool) {
		if len(frags) == 0 && !force {
			return
		}
		lines = append(lines, rawLine{
			Width:     lineWidth,
			Height:    style.LinePx,
			Ascent:    style.Ascent,
			Descent:   style.Descent,
			Fragments: frags,
		})
		frags = nil
		lineWidth = 0
	}

	addFragment := func(text string, widthValue, textWidth float64, fragmentStyle resolvedTextStyle) {
		if text == "" {
			return
		}
		frags = append(frags, layoutFragment{
			Text:      text,
			Width:     widthValue,
			TextWidth: textWidth,
			Style:     fragmentStyle,
		})
		lineWidth += widthValue
	}

	addWhitespace := func(text string) {
		widthValue := float64(len([]rune(text))) * cellWidth
		if lineWidth > 0 && lineWidth+widthValue > width {
			flushLine(false)
		}
		addFragment(text, widthValue, 0, style)
	}

	addText := func(text string, fragmentStyle resolvedTextStyle) {
		textWidth := r.measure(fragmentStyle, text)
		if lineWidth+textWidth <= width {
			addFragment(text, textWidth, textWidth, fragmentStyle)
			return
		}

		if !nearlyZero(lineWidth) {
			available := width - lineWidth
			if available > 0 {
				head, tail := chunkRunesByWidth(text, func(part string) bool {
					return r.measure(fragmentStyle, part) <= available
				})
				headWidth := r.measure(fragmentStyle, head)
				addFragment(head, headWidth, headWidth, fragmentStyle)
				if tail == "" {
					return
				}
				flushLine(false)
				text = tail
				textWidth = r.measure(fragmentStyle, text)
			} else {
				flushLine(false)
			}
			if textWidth <= width {
				addFragment(text, textWidth, textWidth, fragmentStyle)
				return
			}
		} else if textWidth <= width {
			addFragment(text, textWidth, textWidth, fragmentStyle)
			return
		}

		flushLine(false)
		remain := text
		for remain != "" {
			head, tail := chunkRunesByWidth(remain, func(part string) bool {
				return r.measure(fragmentStyle, part) <= width
			})
			headWidth := r.measure(fragmentStyle, head)
			addFragment(head, headWidth, headWidth, fragmentStyle)
			remain = tail
			if remain != "" {
				flushLine(false)
			}
		}
	}

	for _, token := range splitCodeTokens(line) {
		if token == "" {
			continue
		}

		if strings.TrimSpace(token) == "" {
			addWhitespace(token)
			continue
		}

		runes := []rune(token)
		if len(runes) > 0 && isEmojiStart(runes[0]) {
			if end := consumeEmojiCluster(runes, 0); end == len(runes) {
				tokenStyle, err := ensureEmojiStyle()
				if err == nil {
					addText(token, tokenStyle)
					continue
				}
			}
		}

		if useFallback && hasNonASCII(token) {
			addText(token, fallback)
			continue
		}

		addText(token, style)
	}
	flushLine(true)

	if len(lines) == 0 {
		return []rawLine{r.emptyCodeLine(style)}
	}
	return lines
}

func (r *Renderer) emptyCodeLine(style resolvedTextStyle) rawLine {
	return rawLine{
		Height:  style.LinePx,
		Ascent:  style.Ascent,
		Descent: style.Descent,
	}
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
			shift := r.measure(style, "0") * 0.5
			textOffsetX = r.theme.InlineCodePaddingX + shift
			widthValue += r.theme.InlineCodePaddingX*2 + shift
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
			tokenWidth += r.theme.InlineCodePaddingX*2 + r.measure(token.Style, "0")*0.5
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

// draw 根据布局结果真正输出位图。
func (r *Renderer) draw(doc *layoutDocument) (image.Image, error) {
	r.drawContext = gg.NewContext(doc.Width, doc.Height)
	r.drawContext.SetColor(r.theme.Background)
	r.drawContext.Clear()

	for _, block := range doc.Blocks {
		if err := r.drawBlock(r.drawContext, block); err != nil {
			return nil, err
		}
	}

	return r.drawContext.Image(), nil
}

// drawBlock 按块级类型分派到不同绘制逻辑。
func (r *Renderer) drawBlock(dc *gg.Context, block layoutBlock) error {
	switch block.Kind {
	case blockParagraph, blockHeading:
		return r.drawLines(dc, block.Lines)
	case blockCode:
		dc.SetColor(r.theme.CodeFill)
		dc.DrawRoundedRectangle(block.X, block.Y, block.Width, block.Height, r.theme.Radius)
		dc.Fill()
		return r.drawLines(dc, block.Lines)
	case blockRule:
		dc.SetColor(r.theme.Rule)
		dc.SetLineWidth(1.5)
		y := block.Y + block.Height/2
		dc.DrawLine(block.X, y, block.X+block.Width, y)
		dc.Stroke()
		return nil
	case blockImage:
		if block.Image == nil {
			return nil
		}
		return r.drawImageBlock(dc, block)
	case blockBlockquote:
		dc.SetColor(r.theme.QuoteFill)
		dc.DrawRoundedRectangle(block.X, block.Y, block.Width, block.Height, r.theme.Radius)
		dc.Fill()
		dc.SetColor(r.theme.QuoteBar)
		dc.DrawRoundedRectangle(block.X, block.Y, r.theme.QuoteBarWidth, block.Height, r.theme.QuoteBarWidth/2)
		dc.Fill()
		for _, child := range block.Children {
			if err := r.drawBlock(dc, child); err != nil {
				return err
			}
		}
	case blockList:
		for _, item := range block.Items {
			dc.SetColor(item.MarkerStyle.Color)
			if item.UseBullet {
				cx := item.MarkerX + 7
				cy := item.MarkerBaseline - item.MarkerStyle.Ascent*0.35
				dc.DrawCircle(cx, cy, 3.2)
				if item.BulletFilled {
					dc.Fill()
				} else {
					dc.SetLineWidth(1.4)
					dc.Stroke()
				}
			} else {
				dc.SetFontFace(item.MarkerStyle.Face)
				dc.DrawString(item.Marker, item.MarkerX, item.MarkerBaseline)
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

func (r *Renderer) drawImageBlock(dc *gg.Context, block layoutBlock) error {
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
func (r *Renderer) drawLines(dc *gg.Context, lines []layoutLine) error {
	for _, line := range lines {
		for _, frag := range line.Fragments {
			if frag.Style.InlineCode {
				paddingY := r.theme.InlineCodePaddingY
				boxOffsetX := frag.TextOffsetX - r.theme.InlineCodePaddingX
				if boxOffsetX < 0 {
					boxOffsetX = 0
				}
				boxX := frag.X + boxOffsetX
				boxY := line.Baseline - frag.Style.Ascent - paddingY
				boxW := frag.Width - boxOffsetX
				boxH := frag.Style.Ascent + frag.Style.Descent + paddingY*2

				dc.SetColor(r.theme.InlineCode)
				dc.DrawRoundedRectangle(boxX, boxY, boxW, boxH, 6)
				dc.Fill()
			}

			textX := frag.X + frag.TextOffsetX
			dc.SetFontFace(frag.Style.Face)
			dc.SetColor(frag.Style.Color)
			if frag.Style.FauxItalic {
				dc.Push()
				dc.ShearAbout(-0.18, 0, textX, line.Baseline)
			}
			dc.DrawString(frag.Text, textX, line.Baseline)
			if frag.Style.FauxBold {
				dc.DrawString(frag.Text, textX+0.8, line.Baseline)
			}
			if frag.Style.FauxItalic {
				dc.Pop()
			}

			if frag.Style.Underline {
				dc.SetLineWidth(1.2)
				y := line.Baseline + 2
				dc.DrawLine(textX, y, textX+frag.TextWidth, y)
				dc.Stroke()
			}
		}
	}

	return nil
}
