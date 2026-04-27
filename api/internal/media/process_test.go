package media

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"slices"
	"testing"
)

func withOrientation(t *testing.T, jpg []byte, o uint16, bo binary.ByteOrder) []byte {
	t.Helper()
	var tiff bytes.Buffer
	if bo == binary.LittleEndian {
		tiff.WriteString("II")
	} else {
		tiff.WriteString("MM")
	}
	binary.Write(&tiff, bo, uint16(42))
	binary.Write(&tiff, bo, uint32(8))
	binary.Write(&tiff, bo, uint16(1))
	binary.Write(&tiff, bo, uint16(0x0112))
	binary.Write(&tiff, bo, uint16(3))
	binary.Write(&tiff, bo, uint32(1))
	binary.Write(&tiff, bo, o)
	binary.Write(&tiff, bo, uint16(0))
	binary.Write(&tiff, bo, uint32(0))

	seg := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	var out bytes.Buffer
	out.Write(jpg[:2])
	out.Write([]byte{0xFF, 0xE1})
	binary.Write(&out, binary.BigEndian, uint16(len(seg)+2))
	out.Write(seg)
	out.Write(jpg[2:])
	return out.Bytes()
}

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 4), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestJPEGOrientation(t *testing.T) {
	jpg := testJPEG(t, 40, 20)
	if got := jpegOrientation(jpg); got != 1 {
		t.Fatalf("plain jpeg orientation = %d", got)
	}
	for _, bo := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		if got := jpegOrientation(withOrientation(t, jpg, 6, bo)); got != 6 {
			t.Fatalf("%v orientation = %d, want 6", bo, got)
		}
	}
	if got := jpegOrientation([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF}); got != 1 {
		t.Fatalf("truncated segment orientation = %d", got)
	}
}

func TestOrient(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	mark := color.RGBA{255, 0, 0, 255}
	src.Set(0, 0, mark)
	cases := []struct {
		o          int
		w, h, x, y int
	}{
		{1, 3, 2, 0, 0},
		{2, 3, 2, 2, 0},
		{3, 3, 2, 2, 1},
		{4, 3, 2, 0, 1},
		{5, 2, 3, 0, 0},
		{6, 2, 3, 1, 0},
		{7, 2, 3, 1, 2},
		{8, 2, 3, 0, 2},
	}
	for _, c := range cases {
		out := orient(src, c.o)
		b := out.Bounds()
		if b.Dx() != c.w || b.Dy() != c.h {
			t.Errorf("o=%d size = %dx%d, want %dx%d", c.o, b.Dx(), b.Dy(), c.w, c.h)
			continue
		}
		if got := color.RGBAModel.Convert(out.At(c.x, c.y)); got != mark {
			t.Errorf("o=%d mark not at (%d,%d)", c.o, c.x, c.y)
		}
	}
}

func TestProcessRotatesAndStripsMetadata(t *testing.T) {
	data := withOrientation(t, testJPEG(t, 40, 20), 6, binary.BigEndian)
	p, err := process(data)
	if err != nil {
		t.Fatal(err)
	}
	if p.width != 20 || p.height != 40 {
		t.Fatalf("size = %dx%d, want 20x40", p.width, p.height)
	}
	if len(p.variants) != 1 || p.variants[0].Format != "webp" {
		t.Fatalf("variants = %+v", p.variants)
	}
	out := p.files[20]
	if !bytes.HasPrefix(out, []byte("RIFF")) || bytes.Contains(out, []byte("Exif")) {
		t.Fatal("variant should be WebP without EXIF")
	}
}

func TestProcessRejects(t *testing.T) {
	if _, err := process([]byte("GIF89a......")); err != errUnsupportedFormat {
		t.Fatalf("gif err = %v", err)
	}
	if _, err := process([]byte("\x89PNG\r\n\x1a\n garbage")); err != errCorruptImage {
		t.Fatalf("corrupt png err = %v", err)
	}
}

func TestTargetWidths(t *testing.T) {
	cases := map[int][]int{
		300:  {300},
		480:  {480},
		1000: {480, 960, 1000},
		2400: {480, 960, 1600, 2400},
		3024: {480, 960, 1600, 2400},
	}
	for w, want := range cases {
		if got := targetWidths(w); !slices.Equal(got, want) {
			t.Errorf("targetWidths(%d) = %v, want %v", w, got, want)
		}
	}
}

func TestDefaultVariant(t *testing.T) {
	vs := []Variant{{Width: 480}, {Width: 960}, {Width: 1600}, {Width: 2400}}
	if v, _ := defaultVariant(vs); v.Width != 1600 {
		t.Fatalf("default = %d, want 1600", v.Width)
	}
	if v, _ := defaultVariant([]Variant{{Width: 300}}); v.Width != 300 {
		t.Fatalf("default = %d, want 300", v.Width)
	}
	if _, ok := defaultVariant(nil); ok {
		t.Fatal("empty variants should not have default")
	}
}

func TestExtractRefs(t *testing.T) {
	a, b := bytes.Repeat([]byte("a"), 64), bytes.Repeat([]byte("b"), 64)
	body := "![](/media/" + string(a) + ") ![](/media/" + string(b) + "/480.webp) ![](/media/" + string(a) + ") /media/short"
	if got := ExtractRefs(body); !slices.Equal(got, []string{string(a), string(b)}) {
		t.Fatalf("refs = %v", got)
	}
}
