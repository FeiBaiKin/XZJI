package media

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	_ "golang.org/x/image/webp"

	"xzji/internal/dbgen"
	"xzji/internal/platform/storage"
)

func TestVariantKey(t *testing.T) {
	if got := variantKey("2026/10/abcd.png", 480); got != "2026/10/abcd-480.webp" {
		t.Errorf("variantKey = %q", got)
	}
	if got := variantKey("2026/10/abcd", 96); got != "2026/10/abcd-96.webp" {
		t.Errorf("没有扩展名时 variantKey = %q", got)
	}
}

func TestCanMakeVariants(t *testing.T) {
	tests := []struct {
		name          string
		mime          string
		width, height int
		want          bool
	}{
		{"普通图片", "image/png", 800, 600, true},
		{"动图不生成", "image/gif", 800, 600, false},
		{"比最小档还小", "image/png", 64, 64, false},
		{"像素太多", "image/png", 8000, 8000, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canMakeVariants(tt.mime, tt.width, tt.height); got != tt.want {
				t.Errorf("canMakeVariants(%q, %d, %d) = %v，期望 %v", tt.mime, tt.width, tt.height, got, tt.want)
			}
		})
	}
}

// 档位按宽度决定，原图宽度不够的档位跳过，高度按比例缩放。
func TestBuildVariants(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		wantWidths []int
	}{
		{"大图取全部档位", 1200, 800, []int{96, 192, 480, 960}},
		{"中等图取放得下的档位", 500, 250, []int{96, 192, 480}},
		{"小图不放大", 100, 50, []int{96}},
		{"比最小档还小就不生成", 40, 20, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := fill(tt.width, tt.height, func(x, y int) color.Color {
				return color.NRGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255}
			})
			files, err := buildVariants(img, func(w int) string { return variantKey("2026/10/abcd.png", w) })
			if err != nil {
				t.Fatalf("生成缩略图：%v", err)
			}
			gotWidths := make([]int, len(files))
			for i, f := range files {
				gotWidths[i] = f.Width
			}
			if !slices.Equal(gotWidths, tt.wantWidths) {
				t.Fatalf("档位 = %v，期望 %v", gotWidths, tt.wantWidths)
			}
			for _, f := range files {
				// 高度按原图比例，四舍五入到整数
				wantHeight := int(float64(tt.height)*float64(f.Width)/float64(tt.width) + 0.5)
				if f.Height != wantHeight {
					t.Errorf("%d 宽的高度 = %d，期望 %d", f.Width, f.Height, wantHeight)
				}
				cfg, format, err := image.DecodeConfig(bytes.NewReader(f.Data))
				if err != nil {
					t.Fatalf("解码缩略图：%v", err)
				}
				if format != "webp" || cfg.Width != f.Width || cfg.Height != f.Height {
					t.Errorf("缩略图是 %s %dx%d，期望 webp %dx%d", format, cfg.Width, cfg.Height, f.Width, f.Height)
				}
				if !strings.HasSuffix(f.Path, "-"+strconv.Itoa(f.Width)+".webp") {
					t.Errorf("文件名不对：%s", f.Path)
				}
			}
		})
	}
}

// 上传时生成缩略图：文件落盘、写进 variants、对外地址补全。
func TestUploadGeneratesVariants(t *testing.T) {
	dir := t.TempDir()
	store := &fakeStore{}
	svc := NewService(store, storage.NewLocal(dir), "https://img.example.com", 1<<20, testLogger())

	m, err := svc.Upload(context.Background(), 7, bytes.NewReader(pngBytes(t, 1200, 800)))
	if err != nil {
		t.Fatalf("上传：%v", err)
	}
	if len(m.Variants) != 4 {
		t.Fatalf("缩略图数量 = %d，期望 4", len(m.Variants))
	}
	for _, v := range m.Variants {
		if !strings.HasPrefix(v.URL, "https://img.example.com/") || !strings.HasSuffix(v.URL, ".webp") {
			t.Errorf("缩略图地址不对：%s", v.URL)
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(v.Path))); err != nil {
			t.Errorf("缩略图文件没有保存：%v", err)
		}
	}

	// 存库的是路径和尺寸，没有对外地址
	var saved []Variant
	if err := json.Unmarshal(store.rows[0].Variants, &saved); err != nil {
		t.Fatalf("解析 variants：%v", err)
	}
	if len(saved) != 4 || saved[0].Width != 96 || saved[3].Width != 960 || saved[0].Path == "" {
		t.Errorf("存库的 variants 不对：%+v", saved)
	}

	// 缩略图文件比原图小
	orig, err := os.Stat(filepath.Join(dir, filepath.FromSlash(m.Path)))
	if err != nil {
		t.Fatal(err)
	}
	var small int64
	for _, v := range m.Variants {
		if st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(v.Path))); err == nil && v.Width == 480 {
			small = st.Size()
		}
	}
	if small == 0 || small >= orig.Size() {
		t.Errorf("480 宽的缩略图 %d 字节，原图 %d 字节", small, orig.Size())
	}
}

// 补生成：给没有缩略图的图片生成，force 时重新生成已有的。
func TestRebuildVariants(t *testing.T) {
	dir := t.TempDir()
	store := &fakeStore{}
	svc := NewService(store, storage.NewLocal(dir), "https://img.example.com", 1<<20, testLogger())

	if _, err := svc.Upload(context.Background(), 7, bytes.NewReader(pngBytes(t, 600, 400))); err != nil {
		t.Fatalf("上传：%v", err)
	}
	store.rows[0].Variants = nil // 模拟没有缩略图的旧数据

	done, failed, err := svc.RebuildVariants(context.Background(), false)
	if err != nil || done != 1 || failed != 0 {
		t.Fatalf("补生成结果 done=%d failed=%d err=%v", done, failed, err)
	}
	var saved []Variant
	if err := json.Unmarshal(store.rows[0].Variants, &saved); err != nil || len(saved) != 3 {
		t.Fatalf("补生成后的 variants = %s（err=%v）", store.rows[0].Variants, err)
	}

	// 已经有缩略图时默认跳过
	done, _, err = svc.RebuildVariants(context.Background(), false)
	if err != nil || done != 0 {
		t.Fatalf("已有缩略图时应跳过：done=%d err=%v", done, err)
	}

	// 文件缺失时报错但继续处理其他图片
	store.rows = append(store.rows, dbgen.Media{ID: 2, Path: "missing.png", Mime: "image/png"})
	done, failed, err = svc.RebuildVariants(context.Background(), true)
	if err != nil {
		t.Fatalf("补生成不应整体失败：%v", err)
	}
	if done != 1 || failed != 1 {
		t.Errorf("done=%d failed=%d，期望 1 和 1", done, failed)
	}

	// 小图没有档位可生成，不报错
	if _, err := svc.Upload(context.Background(), 7, bytes.NewReader(pngBytes(t, 40, 20))); err != nil {
		t.Fatal(err)
	}
	if v := store.rows[len(store.rows)-1].Variants; v != nil {
		t.Errorf("太小的图片不该有缩略图：%s", v)
	}
}
