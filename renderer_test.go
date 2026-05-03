package gmd

import "testing"

func TestRenderDrawingLibraries(t *testing.T) {
	markdown := []byte(`# Title

Hello **world** and *italic* and ` + "`code`" + `.

- one
- two

` + "```go" + `
func main() {
	fmt.Println("ok")
}
` + "```" + `
`)

	for _, lib := range []DrawingLibrary{DrawingLibraryGG, DrawingLibraryTinySkia} {
		t.Run(string(lib), func(t *testing.T) {
			renderer, err := New(Options{
				Width:          420,
				DrawingLibrary: lib,
			})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			img, err := renderer.Render(markdown)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if img.Bounds().Dx() != 420 || img.Bounds().Dy() <= 0 {
				t.Fatalf("unexpected image bounds: %v", img.Bounds())
			}
		})
	}
}
