package media

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xzji/internal/dbgen"
	"xzji/internal/platform/errs"
	"xzji/internal/platform/storage"
)

func fill(w, h int, f func(x, y int) color.Color) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, f(x, y))
		}
	}
	return img
}

func TestDominantColor(t *testing.T) {
	red := color.NRGBA{R: 255, A: 255}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	clear := color.NRGBA{}
	cases := []struct {
		name string
		img  image.Image
		want string
	}{
		{"纯色", fill(32, 32, func(int, int) color.Color { return red }), "#FF0000"},
		{"白底红标忽略白色", fill(32, 32, func(x, _ int) color.Color {
			if x < 8 {
				return red
			}
			return white
		}), "#FF0000"},
		{"透明背景只看不透明部分", fill(32, 32, func(x, _ int) color.Color {
			if x < 4 {
				return color.NRGBA{B: 255, A: 255}
			}
			return clear
		}), "#0000FF"},
		{"全灰取平均色", fill(16, 16, func(int, int) color.Color { return color.NRGBA{R: 128, G: 128, B: 128, A: 255} }), "#808080"},
		{"全透明", fill(16, 16, func(int, int) color.Color { return clear }), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DominantColor(c.img); got != c.want {
				t.Fatalf("DominantColor = %q，期望 %q", got, c.want)
			}
		})
	}
}

type fakeStore struct {
	rows []dbgen.Media
}

func (f *fakeStore) CreateMedia(_ context.Context, a dbgen.CreateMediaParams) (int64, error) {
	id := int64(len(f.rows) + 1)
	f.rows = append(f.rows, dbgen.Media{
		ID: id, UserID: a.UserID, Path: a.Path, Mime: a.Mime, Size: a.Size,
		Width: a.Width, Height: a.Height, Color: a.Color, Variants: a.Variants,
	})
	return id, nil
}

func (f *fakeStore) UpdateMediaVariants(_ context.Context, a dbgen.UpdateMediaVariantsParams) error {
	f.rows[a.ID-1].Variants = a.Variants
	return nil
}

func (f *fakeStore) ListMediaForVariants(context.Context) ([]dbgen.ListMediaForVariantsRow, error) {
	out := make([]dbgen.ListMediaForVariantsRow, len(f.rows))
	for i, r := range f.rows {
		out[i] = dbgen.ListMediaForVariantsRow{ID: r.ID, Path: r.Path, Mime: r.Mime, Variants: r.Variants}
	}
	return out, nil
}

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func (f *fakeStore) GetMedia(_ context.Context, id int64) (dbgen.Media, error) {
	return f.rows[id-1], nil
}

func (f *fakeStore) ListMediaByIDs(context.Context, []int64) ([]dbgen.Media, error) {
	return f.rows, nil
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, fill(w, h, func(int, int) color.Color { return color.NRGBA{G: 200, A: 255} })); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUpload(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(&fakeStore{}, storage.NewLocal(dir), "https://img.example.com", 1<<20, testLogger())

	m, err := svc.Upload(context.Background(), 7, bytes.NewReader(pngBytes(t, 40, 20)))
	if err != nil {
		t.Fatalf("上传 PNG：%v", err)
	}
	if m.Width != 40 || m.Height != 20 || m.Mime != "image/png" || m.Color != "#00C800" {
		t.Fatalf("图片信息不对：%+v", m)
	}
	if !strings.HasPrefix(m.URL, "https://img.example.com/") || !strings.HasSuffix(m.Path, ".png") {
		t.Fatalf("地址不对：%s", m.URL)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(m.Path))); err != nil {
		t.Fatalf("文件没有保存：%v", err)
	}

	bad := []struct {
		name string
		data []byte
		code string
	}{
		{"空文件", nil, "EMPTY_FILE"},
		{"不是图片", []byte("<?php echo 1; ?>"), "UNSUPPORTED_FILE"},
		{"SVG 不允许", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), "UNSUPPORTED_FILE"},
		{"超过大小", append(pngBytes(t, 1, 1), make([]byte, 1<<20)...), "FILE_TOO_LARGE"},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.Upload(context.Background(), 7, bytes.NewReader(c.data))
			var de *errs.Error
			if !errors.As(err, &de) || de.Code != c.code {
				t.Fatalf("期望 %s，得到 %v", c.code, err)
			}
		})
	}
}
