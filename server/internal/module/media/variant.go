package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"math"
	"path"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"github.com/gen2brain/webp"
	"golang.org/x/image/draw"

	"xzji/internal/dbgen"
)

const (
	// variantQuality WebP 有损压缩质量，缩略图不需要无损
	variantQuality = 80
	// variantMaxPixels 超过这个像素数就不生成缩略图：解码大图会占用几十 MB 内存，
	// 而后端常驻内存目标是不含 MySQL 100MB
	variantMaxPixels = 16_000_000
	// freeMemoryPixels 超过这个像素数的图片生成完缩略图后主动回收内存
	freeMemoryPixels = 2_000_000
)

// variantWidths 缩略图宽度档位：软件图标用 96 / 192，封面用 480 / 960。
// 原图宽度不够的档位直接跳过，不放大。
var variantWidths = []int{96, 192, 480, 960}

// variantMu 同一时刻只处理一张图片：编码大图时每个档位都会占用几十 MB，
// 并发处理会把内存占用成倍放大
var variantMu sync.Mutex

// Variant 一档缩略图。Path 存库，URL 是读取时按 media.base_url 补上的对外地址。
type Variant struct {
	Path   string `json:"path"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	URL    string `json:"-"`
}

// variantFile 一档缩略图的文件内容。
type variantFile struct {
	Variant
	Data []byte
}

// buildVariants 按档位缩放并编码成 WebP，keyOf 决定每个档位的文件名。
func buildVariants(img image.Image, keyOf func(width int) string) ([]variantFile, error) {
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, fmt.Errorf("图片尺寸不正确：%dx%d", b.Dx(), b.Dy())
	}
	variantMu.Lock()
	defer variantMu.Unlock()

	out := make([]variantFile, 0, len(variantWidths))
	for _, w := range variantWidths {
		if w > b.Dx() {
			continue
		}
		h := max(1, int(math.Round(float64(b.Dy())*float64(w)/float64(b.Dx()))))
		scaled := image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.CatmullRom.Scale(scaled, scaled.Bounds(), img, b, draw.Src, nil)

		var buf bytes.Buffer
		if err := webp.Encode(&buf, scaled, webp.Options{Quality: variantQuality}); err != nil {
			return nil, fmt.Errorf("编码 %d 宽的缩略图：%w", w, err)
		}
		out = append(out, variantFile{Variant: Variant{Path: keyOf(w), Width: w, Height: h}, Data: buf.Bytes()})
		// 编码器每个档位会留下一批临时内存，及时回收，避免几个档位叠加
		runtime.GC()
	}
	return out, nil
}

// variantKey 在原文件名后加宽度：2026/10/abcd.webp → 2026/10/abcd-480.webp
func variantKey(baseKey string, width int) string {
	return strings.TrimSuffix(baseKey, path.Ext(baseKey)) + "-" + strconv.Itoa(width) + ".webp"
}

// canMakeVariants 动图取单帧没有意义，GIF 不生成缩略图；像素太多的图解码会占用大量内存，也跳过。
func canMakeVariants(mime string, width, height int) bool {
	if mime == "image/gif" {
		return false
	}
	return width >= variantWidths[0] && width*height <= variantMaxPixels
}

// makeVariants 生成缩略图文件并保存，返回写入数据库的信息。
func (s *Service) makeVariants(ctx context.Context, img image.Image, baseKey string) ([]Variant, error) {
	files, err := buildVariants(img, func(width int) string { return variantKey(baseKey, width) })
	if err != nil {
		return nil, err
	}
	out := make([]Variant, 0, len(files))
	for _, f := range files {
		if err := s.storage.Put(ctx, f.Path, f.Data); err != nil {
			return nil, err
		}
		out = append(out, f.Variant)
	}
	// 大图编码会把堆撑到上百 MB，主动归还给系统，别让一次上传把常驻内存顶上去
	if b := img.Bounds(); b.Dx()*b.Dy() >= freeMemoryPixels {
		debug.FreeOSMemory()
	}
	return out, nil
}

// encodeVariants 序列化后写进 media.variants，没有缩略图时返回 nil（存 NULL）。
func encodeVariants(vs []Variant) []byte {
	if len(vs) == 0 {
		return nil
	}
	b, err := json.Marshal(vs)
	if err != nil {
		return nil
	}
	return b
}

// RebuildVariants 为已有图片补生成缩略图，force 为 true 时连已有的也重新生成。
// 返回处理成功和失败的数量。
func (s *Service) RebuildVariants(ctx context.Context, force bool) (done, failed int, err error) {
	rows, err := s.q.ListMediaForVariants(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("查询上传文件：%w", err)
	}
	for _, r := range rows {
		if len(r.Variants) > 0 && !force {
			continue
		}
		if err := s.rebuildOne(ctx, r); err != nil {
			failed++
			s.log.Warn("补生成缩略图失败", "path", r.Path, "error", err)
			continue
		}
		done++
	}
	return done, failed, nil
}

func (s *Service) rebuildOne(ctx context.Context, r dbgen.ListMediaForVariantsRow) error {
	data, err := s.storage.Get(ctx, r.Path)
	if err != nil {
		return err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ErrUnsupported
	}
	if !canMakeVariants(r.Mime, cfg.Width, cfg.Height) {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ErrUnsupported
	}
	variants, err := s.makeVariants(ctx, img, r.Path)
	if err != nil {
		return err
	}
	return s.q.UpdateMediaVariants(ctx, dbgen.UpdateMediaVariantsParams{Variants: encodeVariants(variants), ID: r.ID})
}
