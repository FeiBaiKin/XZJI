package media

import (
	"fmt"
	"image"
)

// DominantColor 取图片里最显眼的颜色，用作软件图标的底色。
// 按饱和度加权统计色块，透明和接近黑白灰的像素不参与；整张图都是灰色时取平均色，全透明时返回空。
func DominantColor(img image.Image) string {
	type acc struct{ w, r, g, b float64 }
	b := img.Bounds()
	step := max(1, max(b.Dx(), b.Dy())/64)
	buckets := map[int]*acc{}
	var avg acc
	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			r16, g16, b16, a16 := img.At(x, y).RGBA()
			if a16 < 0x8000 {
				continue
			}
			// RGBA() 返回预乘透明度的 16 位值，还原成 0~255
			a := float64(a16)
			r, g, bl := float64(r16)*255/a, float64(g16)*255/a, float64(b16)*255/a
			avg.w++
			avg.r += r
			avg.g += g
			avg.b += bl
			hi, lo := max(r, g, bl), min(r, g, bl)
			if hi < 40 || (hi-lo)/hi < 0.25 {
				continue
			}
			sat := (hi - lo) / hi
			key := int(r)>>5<<6 | int(g)>>5<<3 | int(bl)>>5
			c := buckets[key]
			if c == nil {
				c = &acc{}
				buckets[key] = c
			}
			c.w += sat
			c.r += r * sat
			c.g += g * sat
			c.b += bl * sat
		}
	}
	best := &avg
	for _, c := range buckets {
		if best == &avg || c.w > best.w {
			best = c
		}
	}
	if best.w == 0 {
		return ""
	}
	return fmt.Sprintf("#%02X%02X%02X", uint8(best.r/best.w+0.5), uint8(best.g/best.w+0.5), uint8(best.b/best.w+0.5))
}
