### gmd

`gmd` 是一个纯 Go 的 Markdown 转图片项目。

- Markdown 解析基于 `github.com/yuin/goldmark`
- 绘图基于 `github.com/FloatTech/gg`
- 内置 `default` 和 `github-dark` 两套主题
- 采用“两阶段布局 + 一次性绘制”的渲染流程

### 项目目标

- 尽量遵循 CommonMark 常见语法
- 在保证结构简单的前提下提升性能与稳定性
- 便于继续扩展主题、布局和字体能力

### 已支持的 Markdown 语法

- 标题
- 段落与自动换行
- **粗体**、*斜体*、`行内代码`
- 有序列表与无序列表
- 引用块
- 代码块
- [链接文本]()
- 图片

### 快速运行

渲染内置示例：

```cmd
go run ./cmd
```

渲染指定 Markdown 文件：

```cmd
go run ./cmd -in ./example.md -out ./output/example.png
```

切换到 GitHub 风格深色主题：

```cmd
go run ./cmd -in ./example.md -out ./output/example-dark.png -theme github-dark
```

### 字体说明

- `gmd.Options.Font` 和 `gmd.Options.MonoFont` 支持传入 `TTF / OTF / TTC / OTC` 字体数据
- `gmd.Options.FontIndex` 和 `gmd.Options.MonoFontIndex` 用于选择字体集合中的第几个字体，单字体文件固定为 `0`
- 未传入正文字体时，使用 Go 内置默认字体，内置字体不支持中文
- 未传入代码字体时，使用 Go 内置等宽字体

注意：

- Go 内置默认字体不适合作为完整中文字体使用
- 如果要稳定渲染中文，建议显式传入系统中文字体，例如 Windows 上常见的 `msyh.ttc`、`simsun.ttc`

命令行中可以直接指定字体路径：

```cmd
go run ./cmd -in ./example.md -out ./output/example.png -font .\msyh.ttc -font-index 0
```

如果要覆盖代码块和行内代码的等宽字体：

```cmd
go run ./cmd -in ./example.md -out ./output/example.png -mono-font .\JetBrainsMono-Regular.ttf
```

如果字体文件是集合字体，例如 `ttc` 或 `otc`，可以额外指定索引：

```cmd
go run ./cmd -in ./example.md -out ./output/example.png -font C:\Windows\Fonts\msyh.ttc -font-index 0
```

### Windows 系统字体

`font` 包提供了 `LoadWindowsFont`，可以直接从 Windows 系统字体目录加载字体文件。

示例：

```go
package main

import (
	"log"

	"github.com/lianhong2758/gmd"
	markfont "github.com/lianhong2758/gmd/font"
)

func main() {
	fontdata, err := markfont.LoadWindowsFont("simsun.ttc")
	if err != nil {
		log.Fatal(err)
	}

	r, err := gmd.New(gmd.Options{
		ThemeName: gmd.ThemeDefault,
		Width:     1200,
		Font:      fontData,
		FontIndex: 0,
	})
	if err != nil {
		log.Fatal(err)
	}

	_, err = r.Render([]byte("# 你好，世界"))
	if err != nil {
		log.Fatal(err)
	}
}
```

### 在代码中使用

```go
package main

import (
	"log"
	"os"

	"github.com/lianhong2758/gmd"
)

func main() {
	md, err := os.ReadFile("example.md")
	if err != nil {
		log.Fatal(err)
	}

	r, err := gmd.New(gmd.Options{
		ThemeName: gmd.ThemeGitHubDark,
		Width:     1200,
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := r.RenderToFile(md, "output/example.png"); err != nil {
		log.Fatal(err)
	}

	r.Clear()
}
```
