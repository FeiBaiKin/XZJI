// Package media 处理图片上传：按内容校验真实类型和尺寸、保存文件、提取主色。
package media

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"time"

	_ "golang.org/x/image/webp"

	"xzji/internal/dbgen"
	"xzji/internal/platform/errs"
	"xzji/internal/platform/storage"
)

const (
	// maxSide 单边像素上限，防止解压炸弹
	maxSide = 10000
	// colorMaxPixels 超过这个像素数的图片不提取主色，避免整图解码占用过多内存
	colorMaxPixels = 4_000_000
)

var allowed = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

var (
	ErrNotFound    = errs.NotFound("MEDIA_NOT_FOUND", "图片不存在")
	ErrEmpty       = errs.Invalid("EMPTY_FILE", "请选择要上传的图片")
	ErrUnsupported = errs.Invalid("UNSUPPORTED_FILE", "只支持 JPG、PNG、GIF、WebP 格式的图片")
	ErrDimensions  = errs.Invalid("IMAGE_TOO_LARGE", fmt.Sprintf("图片宽高不能超过 %d 像素", maxSide))
)

type Media struct {
	ID     int64
	UserID int64
	Path   string
	URL    string
	Mime   string
	Size   int64
	Width  int
	Height int
	Color  string
	// Variants 各档缩略图，按宽度从小到大；没有缩略图时为空，前台退回原图
	Variants  []Variant
	CreatedAt time.Time
}

// Store 本模块用到的数据库查询。
type Store interface {
	CreateMedia(ctx context.Context, arg dbgen.CreateMediaParams) (int64, error)
	GetMedia(ctx context.Context, id int64) (dbgen.Media, error)
	ListMediaByIDs(ctx context.Context, ids []int64) ([]dbgen.Media, error)
	UpdateMediaVariants(ctx context.Context, arg dbgen.UpdateMediaVariantsParams) error
	ListMediaForVariants(ctx context.Context) ([]dbgen.ListMediaForVariantsRow, error)
}

type Service struct {
	q       Store
	storage storage.Storage
	baseURL string
	maxSize int64
	log     *slog.Logger
	now     func() time.Time
}

// NewService baseURL 为上传文件的对外地址前缀，末尾不带 /。
func NewService(q Store, st storage.Storage, baseURL string, maxSize int64, log *slog.Logger) *Service {
	return &Service{q: q, storage: st, baseURL: baseURL, maxSize: maxSize, log: log, now: time.Now}
}

func (s *Service) BaseURL() string { return s.baseURL }

func (s *Service) fromRow(r dbgen.Media) Media {
	m := Media{
		ID: r.ID, UserID: r.UserID, Path: r.Path, URL: s.baseURL + "/" + r.Path,
		Mime: r.Mime, Size: r.Size, Width: int(r.Width), Height: int(r.Height), Color: r.Color, CreatedAt: r.CreatedAt,
	}
	if len(r.Variants) > 0 {
		var vs []Variant
		if err := json.Unmarshal(r.Variants, &vs); err != nil {
			s.log.Warn("解析缩略图信息失败", "path", r.Path, "error", err)
		}
		for i := range vs {
			vs[i].URL = s.baseURL + "/" + vs[i].Path
		}
		m.Variants = vs
	}
	return m
}

// Upload 校验并保存一张图片。只读取到大小上限为止，超出即拒绝。
func (s *Service) Upload(ctx context.Context, userID int64, r io.Reader) (Media, error) {
	data, err := io.ReadAll(io.LimitReader(r, s.maxSize+1))
	if err != nil {
		return Media{}, fmt.Errorf("读取上传文件：%w", err)
	}
	if len(data) == 0 {
		return Media{}, ErrEmpty
	}
	if int64(len(data)) > s.maxSize {
		return Media{}, errs.Invalid("FILE_TOO_LARGE", fmt.Sprintf("图片不能超过 %d MB", s.maxSize>>20))
	}
	mime := http.DetectContentType(data)
	ext, ok := allowed[mime]
	if !ok {
		return Media{}, ErrUnsupported
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Media{}, ErrUnsupported
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxSide || cfg.Height > maxSide {
		return Media{}, ErrDimensions
	}

	// 同一张图只解码一次，主色和缩略图共用；解不开时按没有主色、没有缩略图处理
	wantColor, wantVariants := cfg.Width*cfg.Height <= colorMaxPixels, canMakeVariants(mime, cfg.Width, cfg.Height)
	var img image.Image
	if wantColor || wantVariants {
		if decoded, _, err := image.Decode(bytes.NewReader(data)); err == nil {
			img = decoded
		}
	}
	var color string
	if wantColor && img != nil {
		color = DominantColor(img)
	}

	name := make([]byte, 16)
	if _, err := rand.Read(name); err != nil {
		return Media{}, fmt.Errorf("生成文件名：%w", err)
	}
	key := s.now().UTC().Format("2006/01") + "/" + hex.EncodeToString(name) + ext
	if err := s.storage.Put(ctx, key, data); err != nil {
		return Media{}, err
	}
	var variants []Variant
	if wantVariants && img != nil {
		if variants, err = s.makeVariants(ctx, img, key); err != nil {
			s.log.Warn("生成缩略图失败，前台将退回原图", "path", key, "error", err)
			variants = nil
		}
	}
	id, err := s.q.CreateMedia(ctx, dbgen.CreateMediaParams{
		UserID: userID, Path: key, Mime: mime, Size: int64(len(data)),
		Width: int32(cfg.Width), Height: int32(cfg.Height), Color: color, Variants: encodeVariants(variants),
	})
	if err != nil {
		_ = s.storage.Delete(ctx, key)
		return Media{}, fmt.Errorf("记录上传文件：%w", err)
	}
	return s.Get(ctx, id)
}

func (s *Service) Get(ctx context.Context, id int64) (Media, error) {
	row, err := s.q.GetMedia(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Media{}, ErrNotFound
	}
	if err != nil {
		return Media{}, fmt.Errorf("查询图片 %d：%w", id, err)
	}
	return s.fromRow(row), nil
}

// GetMany 批量查询，不存在的 ID 不出现在结果里。
func (s *Service) GetMany(ctx context.Context, ids []int64) (map[int64]Media, error) {
	out := make(map[int64]Media, len(ids))
	uniq := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			uniq = append(uniq, id)
		}
	}
	if len(uniq) == 0 {
		return out, nil
	}
	rows, err := s.q.ListMediaByIDs(ctx, uniq)
	if err != nil {
		return nil, fmt.Errorf("批量查询图片：%w", err)
	}
	for _, r := range rows {
		out[r.ID] = s.fromRow(r)
	}
	return out, nil
}
