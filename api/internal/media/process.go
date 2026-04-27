package media

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"

	"github.com/gen2brain/webp"
	xdraw "golang.org/x/image/draw"
)

const (
	maxPixels   = 40_000_000
	webpQuality = 80
)

var widthLadder = []int{480, 960, 1600, 2400}

var (
	errUnsupportedFormat = errors.New("unsupported image format")
	errTooManyPixels     = errors.New("image has too many pixels")
	errCorruptImage      = errors.New("corrupt image")
)

type processed struct {
	width, height int
	variants      []Variant
	files         map[int][]byte
}

func sniff(data []byte) (string, error) {
	switch ct := http.DetectContentType(data); ct {
	case "image/jpeg", "image/png", "image/webp":
		return ct, nil
	default:
		return "", errUnsupportedFormat
	}
}

// process 解码图片并按 widthLadder 生成不放大的 WebP 变体。
// 先只读取尺寸检查像素上限，再完整解码，避免恶意大图耗尽内存。
func process(data []byte) (*processed, error) {
	ct, err := sniff(data)
	if err != nil {
		return nil, err
	}
	var cfg image.Config
	switch ct {
	case "image/jpeg":
		cfg, err = jpeg.DecodeConfig(bytes.NewReader(data))
	case "image/png":
		cfg, err = png.DecodeConfig(bytes.NewReader(data))
	case "image/webp":
		cfg, err = webp.DecodeConfig(bytes.NewReader(data))
	}
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, errCorruptImage
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return nil, errTooManyPixels
	}

	var img image.Image
	switch ct {
	case "image/jpeg":
		img, err = jpeg.Decode(bytes.NewReader(data))
		// 标准库不处理 EXIF 方向，需要手动旋转；webp 库可通过 AutoRotate 自行处理。
		if err == nil {
			img = orient(toRGBA(img), jpegOrientation(data))
		}
	case "image/png":
		img, err = png.Decode(bytes.NewReader(data))
	case "image/webp":
		img, err = webp.Decode(bytes.NewReader(data), webp.Options{AutoRotate: true})
	}
	if err != nil {
		return nil, errCorruptImage
	}

	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := &processed{width: w, height: h, files: map[int][]byte{}}
	for _, tw := range targetWidths(w) {
		th := max(1, int(float64(h)*float64(tw)/float64(w)+0.5))
		var src image.Image = img
		if tw != w {
			dst := image.NewRGBA(image.Rect(0, 0, tw, th))
			xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Src, nil)
			src = dst
		}
		var buf bytes.Buffer
		if err := webp.Encode(&buf, src, webp.Options{Quality: webpQuality}); err != nil {
			return nil, err
		}
		out.files[tw] = buf.Bytes()
		out.variants = append(out.variants, Variant{Width: tw, Height: th, Format: "webp", Size: int64(buf.Len())})
	}
	return out, nil
}

func targetWidths(w int) []int {
	var out []int
	for _, lw := range widthLadder {
		if lw < w {
			out = append(out, lw)
		}
	}
	if top := widthLadder[len(widthLadder)-1]; w <= top {
		out = append(out, w)
	}
	return out
}

func toRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}

// orient 按 EXIF Orientation（2–8）对像素做翻转/旋转，5–8 会交换宽高。
func orient(src *image.RGBA, o int) image.Image {
	if o < 2 || o > 8 {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range dh {
		for x := range dw {
			var sx, sy int
			switch o {
			case 2:
				sx, sy = w-1-x, y
			case 3:
				sx, sy = w-1-x, h-1-y
			case 4:
				sx, sy = x, h-1-y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, h-1-x
			case 7:
				sx, sy = w-1-y, h-1-x
			case 8:
				sx, sy = w-1-y, x
			}
			si := sy*src.Stride + sx*4
			di := y*dst.Stride + x*4
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return dst
}

// jpegOrientation 在 SOS 之前的 APP1 段中查找 Exif 并读取 Orientation 标签；
// 找不到或解析失败一律返回 1（不旋转）。
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		if marker == 0xD8 || (marker >= 0xD0 && marker <= 0xD7) || marker == 0x01 || marker == 0xFF {
			i += 2
			continue
		}
		if marker == 0xDA || marker == 0xD9 {
			return 1
		}
		segLen := int(binary.BigEndian.Uint16(data[i+2:]))
		if segLen < 2 || i+2+segLen > len(data) {
			return 1
		}
		seg := data[i+4 : i+2+segLen]
		if marker == 0xE1 && len(seg) > 6 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
		i += 2 + segLen
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	if bo.Uint16(t[2:]) != 42 {
		return 1
	}
	off := int(bo.Uint32(t[4:]))
	if off < 8 || off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off:]))
	for k := range n {
		e := off + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 && bo.Uint16(t[e+2:]) == 3 {
			if o := int(bo.Uint16(t[e+8:])); o >= 1 && o <= 8 {
				return o
			}
			return 1
		}
	}
	return 1
}
