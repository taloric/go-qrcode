// go-qrcode
// Copyright 2014 Tom Harwood

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	defaultRecoveryLevel = qrcode.Highest
	contentTooLongErrMsg = "content too long to encode"
)

func main() {
	outFile := flag.String("o", "", "out PNG file prefix, empty for stdout")
	size := flag.Int("s", 256, "image size (pixel)")
	textArt := flag.Bool("t", false, "print as text-art on stdout")
	negative := flag.Bool("i", false, "invert black and white")
	disableBorder := flag.Bool("d", false, "disable QR Code border")
	splitLong := flag.Bool("split-long", false, "split long content into multiple QR codes when necessary")
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

`)
	}
	flag.Parse()

	if len(flag.Args()) == 0 {
		flag.Usage()
		checkError(fmt.Errorf("Error: no content given"))
	}

	content := strings.Join(flag.Args(), " ")

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

	if *splitLong && isContentTooLong(err) {
		checkError(splitAndWrite(content, *size, *outFile, *disableBorder, *negative, *textArt))
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

func splitAndWrite(content string, size int, outPrefix string, disableBorder, negative, textArt bool) error {
	if textArt {
		return errors.New("split-long does not support text-art output")
	}

	if outPrefix == "" {
		return errors.New("split-long requires an output file prefix via -o")
	}

	contentRunes := []rune(content)
	chunkIndex := 0

	for len(contentRunes) > 0 {
		chunkLen, err := maxEncodablePrefix(contentRunes)
		if err != nil {
			return err
		}

		chunk := string(contentRunes[:chunkLen])
		contentRunes = contentRunes[chunkLen:]

		q, err := prepareQRCode(chunk, disableBorder)
		if err != nil {
			return err
		}

		if negative {
			q.ForegroundColor, q.BackgroundColor = q.BackgroundColor, q.ForegroundColor
		}

		png, err := q.PNG(size)
		if err != nil {
			return err
		}

		filename := fmt.Sprintf("%s-%d.png", outPrefix, chunkIndex)
		if err := writeFile(filename, png); err != nil {
			return err
		}

		chunkIndex++
	}

	return nil
}

func maxEncodablePrefix(content []rune) (int, error) {
	low, high := 1, len(content)
	best := 0

	for low <= high {
		mid := (low + high) / 2
		_, err := qrcode.New(string(content[:mid]), defaultRecoveryLevel)

		if err == nil {
			best = mid
			low = mid + 1
			continue
		}

		if !isContentTooLong(err) {
			return 0, err
		}

		high = mid - 1
	}

	if best == 0 {
		return 0, errors.New("content segment too long to encode")
	}

	return best, nil
}

func isContentTooLong(err error) bool {
	return err != nil && err.Error() == contentTooLongErrMsg
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
