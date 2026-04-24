package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lianhong2758/gmd"
	markfont "github.com/lianhong2758/gmd/font"
)

const sampleMarkdown = `# Markdown 转图片示例

这是一个基于 **goldmark** 和 ` + "`FloatTech/gg`" + ` 的纯 Go Markdown 图片渲染器。

它支持以下常见语法：

- 标题
- 段落与自动换行
- **粗体**、*斜体*、` + "`行内代码`" + `
- 有序列表与无序列表
- 引用块
- 代码块
- [链接文本](https://github.com/FloatTech/gg)

> 设计重点不是“把 AST 直接画出来”，而是先做轻量结构转换，再统一布局，这样性能和可维护性都更好。

## 代码块

` + "```go" + `
func main() {
	r, err := marktoimage.New(marktoimage.Options{
		ThemeName: marktoimage.ThemeGitHubDark,
		Width:     1200,
		Fonts:     [][]byte{markfont.TTF},
	})
	if err != nil {
		fmt.Printf("初始化渲染器失败: %v\n", err)
		return
	}
	if err := r.RenderToFile([]byte(sampleMarkdown), "markdown.png"); err != nil {
		fmt.Printf("生成图片失败: %v\n", err)
		return
	}

	fmt.Printf("已生成图片: %s\n", "markdown.png")
}

` + "```" + `
---
![RUNOOB 图标](https://static.jyshare.com/images/runoob-logo.png)
1. 先解析 Markdown
2. 再根据宽度完成布局
3. 最后一次性绘制到 PNG`

func main() {
	var (
		in    = flag.String("in", "", "Markdown 文件路径，留空则使用内置示例")
		out   = flag.String("out", "output/markdown.png", "输出 PNG 路径")
		width = flag.Int("width", 1200, "图片宽度")
		font  = flag.String("font", "", "外部 TTF 字体文件路径，设置后优先使用")
		theme = flag.String("theme", string(gmd.ThemeDefault), "内置主题，可选: "+strings.Join(gmd.ThemeNames(), ", "))
	)
	flag.Parse()

	content := []byte(sampleMarkdown)
	if *in != "" {
		data, err := os.ReadFile(*in)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取 Markdown 文件失败: %v\n", err)
			os.Exit(1)
		}
		content = data
	}

	themeName, err := gmd.ParseThemeName(*theme)
	if err != nil {
		fmt.Fprintf(os.Stderr, "主题配置无效: %v\n", err)
		os.Exit(1)
	}
	fontdata := [][]byte{}
	if *font != "" {
		b, err := os.ReadFile(*font)
		if err != nil {
			fmt.Fprintf(os.Stderr, "字体配置无效: %v\n", err)
			os.Exit(1)
		}
		fontdata = append(fontdata, b)
	} else {
		fontdata = append(fontdata, markfont.TTF)
	}

	r, err := gmd.New(gmd.Options{
		ThemeName: themeName,
		Width:     *width,
		Fonts:     fontdata,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化渲染器失败: %v\n", err)
		os.Exit(1)
	}
	output := filepath.Clean(*out)
	if err := r.RenderToFile(content, output); err != nil {
		fmt.Fprintf(os.Stderr, "生成图片失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("已生成图片: %s\n", output)
}
