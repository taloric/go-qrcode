// go-qrcode
// Copyright 2014 Tom Harwood

package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
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

  4. Decode QR codes from a file or directory (requires zbarimg installed):

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
	text, err := zbarimgDecode(path)
	if err != nil {
		return fmt.Errorf("decode %s: %w", filepath.Base(path), err)
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

	decoded := 0
	failed := 0

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
		text, err := zbarimgDecode(imgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to decode %s: %v\n", name, err)
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

	fmt.Fprintf(os.Stderr, "Done: %d decoded, %d failed\n", decoded, failed)
	return nil
}

// zbarimgDecode decodes a QR code from an image file using zbarimg.
// Requires zbarimg installed (apt: zbar-tools, brew: zbar).
func zbarimgDecode(path string) (string, error) {
	cmd := exec.Command("zbarimg", "--quiet", "--raw", "-Sdisable", "-Sqrcode.enable", path)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("zbarimg: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	// zbarimg --raw outputs the decoded text followed by a newline
	result := out.String()
	result = strings.TrimSuffix(result, "\n")
	return result, nil
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
