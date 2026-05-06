### gmd

gmd是一个纯 Go 的 Markdown 转图片项目

- Markdown 解析基于 `github.com/yuin/goldmark`
- 绘图基于 `github.com/FloatTech/gg`
- 内置 `default` 和 `github-dark` 两套主题，可通过配置切换

### 项目目标：

- 尽量遵循 CommonMark 常见语法
- 通过“两阶段布局 + 一次性绘制”提升性能与稳定性
- 保持结构简单，便于继续扩展

### 已支持的Markdown语法:

- 标题
- 段落与自动换行
- **粗体** 、*斜体* 、`行内代码`
- 有序列表与无序列表
- 引用块
- 代码块
- [链接文本]()
- 图片


### 快速运行：
渲染示例文本:

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

### 字体说明：

- `font` 中已经静态嵌入了一份中文字体，CLI 默认会启用它。
- 如果你想覆盖内置正文字体，也可以显式指定外部字体文件：
```cmd
go run ./cmd -in ./example.md -out ./output/example.png -font  example.ttf
```
- 如果你想覆盖代码块和行内代码的等宽字体，可以指定：
```cmd
go run ./cmd -in ./example.md -out ./output/example.png -mono-font mono.ttf
```
- 你可以在`gmd.Options.Font`中传入正文字体，在`gmd.Options.MonoFont`中传入代码字体；未传入时会使用默认字体。未传入代码字体时，非 ASCII 代码片段会回退到正文字体。


### 在你的代码中使用gmd

```go
import (
	"github.com/lianhong2758/gmd"
	"github.com/lianhong2758/gmd/font"
)

	r, err := gmd.New(gmd.Options{
		ThemeName: gmd.ThemeGitHubDark,
		Width:     1200,
		Font:      font.TTF,
	})
	img, err := r.Render(mdbyte)
```
