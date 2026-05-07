package gmd

import (
	"image/color"
	"strings"
	"unsafe"
)

// Theme 定义整体视觉风格、间距和颜色方案。
type Theme struct {
	Width              int
	Padding            float64
	BlockGap           float64
	ParagraphGap       float64
	ListItemGap        float64
	QuotePaddingX      float64
	QuotePaddingY      float64
	QuoteBarWidth      float64
	ListIndent         float64
	CodePaddingX       float64
	CodePaddingY       float64
	InlineCodePaddingX float64
	InlineCodePaddingY float64
	RuleSpacing        float64
	BaseFontSize       float64
	BaseLineHeight     float64
	CodeLineHeight     float64
	HeadingScale       [6]float64
	Radius             float64

	Background color.Color
	Text       color.Color
	MutedText  color.Color
	Link       color.Color
	Rule       color.Color
	QuoteBar   color.Color
	QuoteFill  color.Color
	CodeFill   color.Color
	CodeText   color.Color
	CodeLabel  color.Color
	InlineCode color.Color
	InlineText color.Color
}

// ThemeNames 返回所有可用的内置主题名，方便给 CLI 或上层配置展示帮助信息。
func ThemeNames() []string {
	return *(*[]string)(unsafe.Pointer(&builtinThemeOrder))
}

// NormalizeThemeName 把外部输入转换成内部使用的规范主题名。
func NormalizeThemeName(name string) ThemeName {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return ThemeDefault
	}
	return ThemeName(normalized)
}

// ParseThemeName 判断主题名是否存在。
func ParseThemeName(name string) bool {
	_, ok := builtinThemeConfigs[NormalizeThemeName(name)]
	return ok
}

// ThemeByName 根据内置主题名构造完整主题配置。
func ThemeByName(name ThemeName, width int) Theme {
	if width <= 0 {
		width = 1200
	}

	config, ok := builtinThemeConfigs[name]
	if !ok {
		config = builtinThemeConfigs[ThemeDefault]
	}
	config.Width = width
	return config
}

// DefaultTheme 保留旧接口，默认返回浅色主题。
func DefaultTheme(width int) Theme {
	return ThemeByName(ThemeDefault, width)
}

// GitHubDarkTheme 保留旧接口，映射到新的深色主题集合。
func GitHubDarkTheme(width int) Theme {
	return ThemeByName(ThemeDarkGraphite, width)
}

// VSCodeDarkTheme 保留旧接口，映射到新的深色主题集合。
func VSCodeDarkTheme(width int) Theme {
	return ThemeByName(ThemeCobalt, width)
}
