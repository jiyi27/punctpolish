package fileutil

import (
	"bytes"
	"os"
)

const (
	// sniffSize is the number of bytes read to decide whether a file is text.
	sniffSize = 8 * 1024

	// maxNullFraction is the maximum fraction of null bytes allowed in a text file.
	maxNullFraction = 0.01
)

// IsTextFile returns true when the file at path looks like a UTF-8 text file.
// It reads up to sniffSize bytes and checks for binary content heuristics:
//   - Any null byte (0x00) beyond the BOM is a strong binary indicator.
//   - If more than maxNullFraction of the sample bytes are control characters
//     (excluding common text controls), the file is considered binary.
func IsTextFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, sniffSize)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return false, err
	}
	buf = buf[:n]

	// Strip UTF-8 BOM if present so it doesn't skew analysis.
	buf = stripBOM(buf)

	// Null bytes are a reliable binary signal.
	if bytes.IndexByte(buf, 0x00) != -1 {
		return false, nil
	}

	// Count suspicious control characters (not tab, LF, CR, or form-feed).
	suspicious := 0
	for _, b := range buf {
		if b < 0x08 || (b > 0x0D && b < 0x1B) || (b == 0x1C || b == 0x1D || b == 0x1E || b == 0x1F) {
			suspicious++
		}
	}

	if len(buf) > 0 && float64(suspicious)/float64(len(buf)) > maxNullFraction {
		return false, nil
	}

	return true, nil
}

// stripBOM removes a leading UTF-8 byte order mark (EF BB BF) if present.
func stripBOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}
