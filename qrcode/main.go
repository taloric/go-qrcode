// go-qrcode
// Copyright 2014 Tom Harwood

package main

import (
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/makiuchi-d/gozxing"
	gozxingqr "github.com/makiuchi-d/gozxing/qrcode"
	xdraw "golang.org/x/image/draw"
)

const defaultRecoveryLevel = qrcode.Highest

func main() {
	outFile := flag.String("o", "", "out PNG file prefix, empty for stdout")
	size := flag.Int("s", 256, "image size (pixel)")
	textArt := flag.Bool("t", false, "print as text-art on stdout")
	negative := flag.Bool("i", false, "invert black and white")
	disableBorder := flag.Bool("d", false, "disable QR Code border")
	inputFile := flag.String("f", "", "read input from file, hex-encode bytes to text before generating QR")
	splitLong := flag.Bool("split-long", false, "split long content into multiple QR codes")
	grid := flag.Bool("grid", false, "combine split QR codes into a single grid image (use with -split-long)")
	decodePath := flag.String("decode", "", "decode QR code(s) from a file or directory, save as .txt")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `qrcode -- QR Code encoder in Go
https://github.com/skip2/go-qrcode

Flags:
`)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Usage:
  1. Arguments except for flags are joined by " " and used to generate QR code.
     Default output is STDOUT, pipe to imagemagick command "display" to display
     on any X server.

       qrcode hello word | display

  2. Save to file if "display" not available:

       qrcode "homepage: https://github.com/skip2/go-qrcode" > out.png

  3. Encode a binary file as hex, split into multiple QR codes:

       qrcode -f data.csv -split-long -o output

  4. Decode QR codes from a file or directory:

       qrcode -decode ./output-dir
       qrcode -decode image.png
`)
	}
	flag.Parse()

	if *decodePath != "" {
		checkError(decodePNG(*decodePath))
		return
	}

	content, err := loadContent(flag.Args(), *inputFile)
	if err != nil {
		flag.Usage()
		checkError(err)
	}

	q, err := prepareQRCode(content, *disableBorder)

	if err == nil {
		if *textArt {
			art := q.ToString(*negative)
			fmt.Println(art)
			return
		}

		if *negative {
			q.ForegroundColor, q.BackgroundColor = q.BackgroundColor, q.ForegroundColor
		}

		checkError(writeSingleCode(q, *size, *outFile))
		return
	}

	if *splitLong {
		checkError(splitAndWrite(content, *size, *outFile, *disableBorder, *negative, *textArt, *grid))
		return
	}

	checkError(err)
}

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func prepareQRCode(content string, disableBorder bool) (*qrcode.QRCode, error) {
	q, err := qrcode.New(content, defaultRecoveryLevel)
	if err != nil {
		return nil, err
	}

	if disableBorder {
		q.DisableBorder = true
	}

	return q, nil
}

func writeSingleCode(q *qrcode.QRCode, size int, outFile string) error {
	png, err := q.PNG(size)
	if err != nil {
		return err
	}

	if outFile == "" {
		_, err = os.Stdout.Write(png)
		return err
	}

	return writeFile(outFile+".png", png)
}

func splitAndWrite(content string, size int, outPrefix string, disableBorder, negative, textArt, grid bool) error {
	if textArt {
		return errors.New("split-long does not support text-art output")
	}

	if outPrefix == "" {
		return errors.New("split-long requires an output file prefix via -o")
	}

	codes, err := qrcode.EncodeMulti(content, defaultRecoveryLevel)
	if err != nil {
		return err
	}

	for _, q := range codes {
		if disableBorder {
			q.DisableBorder = true
		}
		if negative {
			q.ForegroundColor, q.BackgroundColor = q.BackgroundColor, q.ForegroundColor
		}
	}

	if grid {
		png, err := qrcode.GridPNG(codes, size, 0)
		if err != nil {
			return err
		}
		return writeFile(outPrefix+"-grid.png", png)
	}

	for i, q := range codes {
		png, err := q.PNG(size)
		if err != nil {
			return err
		}
		filename := fmt.Sprintf("%s-%d.png", outPrefix, i)
		if err := writeFile(filename, png); err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stderr, "Split into %d QR codes\n", len(codes))
	return nil
}

func decodePNG(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return decodePNGDir(path)
	}
	return decodePNGFile(path)
}

func decodePNGFile(path string) error {
	reader := gozxingqr.NewQRCodeReader()
	text, err := decodeQRFile(reader, path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n%v\n", filepath.Base(path), err)
		return err
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	txtPath := base + ".txt"
	if err := os.WriteFile(txtPath, []byte(text), 0644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "decoded %s -> %s\n", filepath.Base(path), filepath.Base(txtPath))
	return nil
}

func decodePNGDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("cannot read directory %s: %w", dir, err)
	}

	reader := gozxingqr.NewQRCodeReader()
	decoded := 0
	failed := 0
	var failedFiles []struct {
		name string
		err  error
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
			continue
		}

		imgPath := filepath.Join(dir, name)
		text, err := decodeQRFile(reader, imgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to decode %s:\n%v\n", name, err)
			failedFiles = append(failedFiles, struct {
				name string
				err  error
			}{name, err})
			failed++
			continue
		}

		base := strings.TrimSuffix(name, filepath.Ext(name))
		txtPath := filepath.Join(dir, base+".txt")
		if err := os.WriteFile(txtPath, []byte(text), 0644); err != nil {
			return fmt.Errorf("cannot write %s: %w", txtPath, err)
		}

		fmt.Fprintf(os.Stderr, "decoded %s -> %s\n", name, base+".txt")
		decoded++
	}

	// Generate failure report if there are failed files
	if len(failedFiles) > 0 {
		reportPath := filepath.Join(dir, "decode-failed.txt")
		var report strings.Builder
		report.WriteString(fmt.Sprintf("QR Code Decode Failure Report\n"))
		report.WriteString(fmt.Sprintf("Generated: %s\n", filepath.Base(dir)))
		report.WriteString(fmt.Sprintf("Total Failed: %d\n\n", len(failedFiles)))
		report.WriteString(strings.Repeat("=", 80) + "\n\n")

		for i, f := range failedFiles {
			report.WriteString(fmt.Sprintf("[%d] File: %s\n", i+1, f.name))
			report.WriteString(fmt.Sprintf("Error:\n%v\n", f.err))
			report.WriteString(strings.Repeat("-", 80) + "\n\n")
		}

		if err := os.WriteFile(reportPath, []byte(report.String()), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write failure report: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "\nFailure report written to: %s\n", reportPath)
		}
	}

	fmt.Fprintf(os.Stderr, "Done: %d decoded, %d failed\n", decoded, failed)
	return nil
}

type decodeAttempt struct {
	strategy string
	width    int
	height   int
	err      error
}

func tryDecodeWithStrategies(reader gozxing.Reader, img image.Image) (string, []decodeAttempt, error) {
	var attempts []decodeAttempt
	bounds := img.Bounds()
	origW, origH := bounds.Dx(), bounds.Dy()

	// Strategy 1: Try original size
	if text, err := tryDecode(reader, img); err == nil {
		attempts = append(attempts, decodeAttempt{"original", origW, origH, nil})
		return text, attempts, nil
	} else {
		attempts = append(attempts, decodeAttempt{"original", origW, origH, err})
	}

	// Strategy 2-6: Try different scale factors
	for _, factor := range []int{2, 3, 4, 5} {
		scaled := scaleImageByFactor(img, factor)
		w, h := scaled.Bounds().Dx(), scaled.Bounds().Dy()
		strategy := fmt.Sprintf("scale %dx", factor)

		if text, err := tryDecode(reader, scaled); err == nil {
			attempts = append(attempts, decodeAttempt{strategy, w, h, nil})
			return text, attempts, nil
		} else {
			attempts = append(attempts, decodeAttempt{strategy, w, h, err})
		}

		// Try enhanced version
		enhanced := enhanceImage(scaled)
		strategyEnh := fmt.Sprintf("scale %dx + enhance", factor)
		if text, err := tryDecode(reader, enhanced); err == nil {
			attempts = append(attempts, decodeAttempt{strategyEnh, w, h, nil})
			return text, attempts, nil
		} else {
			attempts = append(attempts, decodeAttempt{strategyEnh, w, h, err})
		}
	}

	return "", attempts, fmt.Errorf("all decode strategies failed")
}

func tryDecode(reader gozxing.Reader, img image.Image) (string, error) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", err
	}
	result, err := reader.Decode(bmp, nil)
	if err != nil {
		return "", err
	}
	return result.GetText(), nil
}

func decodeQRFile(reader gozxing.Reader, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", fmt.Errorf("image decode: %w", err)
	}

	text, attempts, err := tryDecodeWithStrategies(reader, img)
	if err != nil {
		// Build detailed error message
		var errMsg strings.Builder
		errMsg.WriteString(fmt.Sprintf("failed after %d attempts:\n", len(attempts)))
		for i, attempt := range attempts {
			errMsg.WriteString(fmt.Sprintf("  [%d] %s (%dx%d): %v\n", i+1, attempt.strategy, attempt.width, attempt.height, attempt.err))
		}
		return "", fmt.Errorf("%s", errMsg.String())
	}

	return text, nil
}

// scaleUpForDecode scales up small QR code images for reliable decoding.
// Version 40 QR codes have 177x177 modules; gozxing needs ~3px/module minimum.
func scaleUpForDecode(img image.Image) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	const minDim = 2048
	if w >= minDim && h >= minDim {
		return img
	}
	scale := (minDim + w - 1) / w
	if s := (minDim + h - 1) / h; s > scale {
		scale = s
	}
	newW, newH := w*scale, h*scale
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	xdraw.NearestNeighbor.Scale(dst, dst.Bounds(), img, bounds, xdraw.Over, nil)
	return dst
}

// scaleImageByFactor scales an image by a specific factor.
func scaleImageByFactor(img image.Image, factor int) image.Image {
	if factor <= 1 {
		return img
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	newW, newH := w*factor, h*factor
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	xdraw.NearestNeighbor.Scale(dst, dst.Bounds(), img, bounds, xdraw.Over, nil)
	return dst
}

// enhanceImage applies contrast enhancement to improve QR code readability.
func enhanceImage(img image.Image) image.Image {
	bounds := img.Bounds()
	enhanced := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			gray := (r + g + b) / 3
			// Apply contrast enhancement: if closer to white, make whiter; if closer to black, make blacker
			if gray > 32768 {
				gray = 65535
			} else {
				gray = 0
			}
			enhanced.Set(x, y, color.Gray16{uint16(gray)})
			_ = a // preserve alpha if needed
		}
	}
	return enhanced
}

func writeFile(filename string, data []byte) error {
	fh, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer fh.Close()

	_, err = fh.Write(data)
	return err
}

func loadContent(args []string, inputFile string) (string, error) {
	if inputFile == "" {
		if len(args) == 0 {
			return "", fmt.Errorf("Error: no content given")
		}
		return strings.Join(args, " "), nil
	}

	if len(args) > 0 {
		return "", fmt.Errorf("Error: use either -f or arguments, not both")
	}

	data, err := os.ReadFile(inputFile)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(data), nil
}
