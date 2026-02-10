// go-qrcode
// Copyright 2014 Tom Harwood

package qrcode

import "unicode/utf8"

// SplitContent splits content into chunks by byte boundary, each fitting in a
// single QR code at the given recovery level.
func SplitContent(content string, level RecoveryLevel) []string {
	cap := MaxByteCapacity(level)
	if cap <= 0 {
		return nil
	}
	// Reduce capacity by 10 bytes as safety margin
	cap -= 50
	if cap <= 0 {
		return nil
	}

	data := []byte(content)
	var chunks []string
	for len(data) > 0 {
		end := cap
		if end > len(data) {
			end = len(data)
		}
		chunks = append(chunks, string(data[:end]))
		data = data[end:]
	}
	return chunks
}

// SplitContentUTF8 splits content into chunks at rune boundaries, each fitting
// in a single QR code at the given recovery level. This avoids splitting
// multi-byte UTF-8 characters.
func SplitContentUTF8(content string, level RecoveryLevel) []string {
	cap := MaxByteCapacity(level)
	if cap <= 0 {
		return nil
	}
	// Reduce capacity by 10 bytes as safety margin to avoid encoding edge cases
	cap -= 50
	if cap <= 0 {
		return nil
	}

	var chunks []string
	for len(content) > 0 {
		end := cap
		if end > len(content) {
			end = len(content)
		}
		// Back up to rune boundary if we split a multi-byte rune
		for end > 0 && end < len(content) && !utf8.RuneStart(content[end]) {
			end--
		}
		if end == 0 {
			break
		}
		chunks = append(chunks, content[:end])
		content = content[end:]
	}
	return chunks
}
