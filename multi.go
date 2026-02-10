// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
)

// EncodeMulti encodes content that may exceed single QR code capacity.
// Returns a slice of QRCode objects, one per chunk.
func EncodeMulti(content string, level RecoveryLevel) ([]*QRCode, error) {
	chunks := SplitContentUTF8(content, level)
	codes := make([]*QRCode, 0, len(chunks))
	for _, chunk := range chunks {
		q, err := New(chunk, level)
		if err != nil {
			return nil, err
		}
		codes = append(codes, q)
	}
	return codes, nil
}

// GridImage arranges multiple QR code images into a single grid image.
// size is the pixel size per individual QR code.
// cols specifies the number of columns; 0 means auto (square-ish layout).
func GridImage(codes []*QRCode, size int, cols int) image.Image {
	n := len(codes)
	if n == 0 {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}
	if cols <= 0 {
		cols = int(math.Ceil(math.Sqrt(float64(n))))
	}
	rows := (n + cols - 1) / cols

	totalW := cols * size
	totalH := rows * size

	dst := image.NewRGBA(image.Rect(0, 0, totalW, totalH))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	for i, q := range codes {
		r := i / cols
		c := i % cols
		img := q.Image(size)
		dp := image.Point{c * size, r * size}
		rect := image.Rect(dp.X, dp.Y, dp.X+size, dp.Y+size)
		draw.Draw(dst, rect, img, image.Point{}, draw.Over)
	}
	return dst
}

// GridPNG returns the grid image as PNG bytes.
func GridPNG(codes []*QRCode, size int, cols int) ([]byte, error) {
	img := GridImage(codes, size, cols)
	var b bytes.Buffer
	err := png.Encode(&b, img)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
