package gmd

import (
	"fmt"
	"image/color"
	"strings"
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
	InlineCode color.Color
}

// ThemeName 是内置主题的配置名。
type ThemeName string

const (
	ThemeDefault    ThemeName = "default"
	ThemeGitHubDark ThemeName = "github-dark"
)

// ThemeNames 返回所有可用的内置主题名，方便给 CLI 或上层配置展示帮助信息。
func ThemeNames() []string {
	return []string{string(ThemeDefault), string(ThemeGitHubDark)}
}

// ParseThemeName 把外部输入规范化成受支持的主题名。
func ParseThemeName(name string) (ThemeName, error) {
	normalized := ThemeName(strings.ToLower(strings.TrimSpace(name)))
	if normalized == "" {
		return ThemeDefault, nil
	}

	switch normalized {
	case ThemeDefault, ThemeGitHubDark:
		return normalized, nil
	default:
		return "", fmt.Errorf("unknown theme %q, supported themes: %s", name, strings.Join(ThemeNames(), ", "))
	}
}

// ThemeByName 根据内置主题名构造完整主题配置。
func ThemeByName(name ThemeName, width int) Theme {
	if width <= 0 {
		width = 1200
	}

	switch name {
	case ThemeGitHubDark:
		return Theme{
			Width:              width,
			Padding:            48,
			BlockGap:           24,
			ParagraphGap:       18,
			ListItemGap:        8,
			QuotePaddingX:      18,
			QuotePaddingY:      14,
			QuoteBarWidth:      5,
			ListIndent:         34,
			CodePaddingX:       20,
			CodePaddingY:       18,
			InlineCodePaddingX: 8,
			InlineCodePaddingY: 4,
			RuleSpacing:        18,
			BaseFontSize:       22,
			BaseLineHeight:     1.55,
			CodeLineHeight:     1.45,
			HeadingScale:       [6]float64{1.80, 1.55, 1.35, 1.20, 1.10, 1.00},
			Radius:             12,
			Background:         rgb(13, 17, 23),
			Text:               rgb(230, 237, 243),
			MutedText:          rgb(139, 148, 158),
			Link:               rgb(88, 166, 255),
			Rule:               rgb(48, 54, 61),
			QuoteBar:           rgb(63, 185, 80),
			QuoteFill:          rgb(22, 27, 34),
			CodeFill:           rgb(22, 27, 34),
			CodeText:           rgb(230, 237, 243),
			InlineCode:         rgb(45, 51, 59),
		}
	default:
		return Theme{
			Width:              width,
			Padding:            48,
			BlockGap:           24,
			ParagraphGap:       18,
			ListItemGap:        8,
			QuotePaddingX:      18,
			QuotePaddingY:      14,
			QuoteBarWidth:      5,
			ListIndent:         34,
			CodePaddingX:       20,
			CodePaddingY:       18,
			InlineCodePaddingX: 8,
			InlineCodePaddingY: 4,
			RuleSpacing:        18,
			BaseFontSize:       22,
			BaseLineHeight:     1.55,
			CodeLineHeight:     1.45,
			HeadingScale:       [6]float64{1.80, 1.55, 1.35, 1.20, 1.10, 1.00},
			Radius:             14,
			Background:         rgb(248, 250, 252),
			Text:               rgb(15, 23, 42),
			MutedText:          rgb(71, 85, 105),
			Link:               rgb(37, 99, 235),
			Rule:               rgb(203, 213, 225),
			QuoteBar:           rgb(148, 163, 184),
			QuoteFill:          rgb(241, 245, 249),
			CodeFill:           rgb(15, 23, 42),
			CodeText:           rgb(226, 232, 240),
			InlineCode:         rgb(226, 232, 240),
		}
	}
}

// DefaultTheme 保留旧接口，默认返回浅色主题。
func DefaultTheme(width int) Theme {
	return ThemeByName(ThemeDefault, width)
}

// GitHubDarkTheme 提供偏 GitHub Markdown 的深色主题。
func GitHubDarkTheme(width int) Theme {
	return ThemeByName(ThemeGitHubDark, width)
}
