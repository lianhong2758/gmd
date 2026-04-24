package gmd

import (
	"bufio"
	"fmt"
	"image"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

func (r *Renderer) loadImage(src string) (image.Image, error) {
	if strings.TrimSpace(src) == "" {
		return nil, fmt.Errorf("empty image source")
	}

	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Get(src)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status: %s", resp.Status)
		}

		img, _, err := image.Decode(bufio.NewReader(resp.Body))
		if err != nil {
			return nil, err
		}
		return normalizeImage(img), nil
	}

	file, err := os.Open(r.resolveImagePath(src))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(bufio.NewReader(file))
	if err != nil {
		return nil, err
	}
	return normalizeImage(img), nil
}

func (r *Renderer) resolveImagePath(src string) string {
	src = filepath.FromSlash(strings.TrimSpace(src))
	if filepath.IsAbs(src) || r.baseDir == "" {
		return filepath.Clean(src)
	}
	return filepath.Join(r.baseDir, src)
}

func scaleImage(src image.Image, width, height int) image.Image {
	if width <= 0 || height <= 0 {
		return src
	}
	if src.Bounds().Dx() == width && src.Bounds().Dy() == height {
		return src
	}

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

func normalizeImage(src image.Image) image.Image {
	bounds := src.Bounds()
	if bounds.Min.X == 0 && bounds.Min.Y == 0 {
		return src
	}

	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	xdraw.Draw(dst, dst.Bounds(), src, bounds.Min, xdraw.Src)
	return dst
}

func sameSize(a, b float64) bool {
	const tolerance = 0.5
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= tolerance
}
