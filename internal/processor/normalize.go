package processor

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type lineTransform func(string) string

var textLineTransforms = []lineTransform{
	replaceChinesePunctuation,
	normalizeCommaSpacing,
	insertCJKLatinSpacesSafe,
	stripTrailingPeriodAndComma,
}

// NormalizeText processes only normal Markdown text and leaves fenced code
// blocks untouched.
func NormalizeText(input string) string {
	lines := strings.Split(input, "\n")
	normalized := make([]string, len(lines))

	inFence := false
	fenceMarker := ""

	for i, line := range lines {
		marker, isFence := detectFenceMarker(line)
		if isFence {
			normalized[i] = line
			if !inFence {
				inFence = true
				fenceMarker = marker
				continue
			}
			if marker == fenceMarker {
				inFence = false
				fenceMarker = ""
			}
			continue
		}

		if inFence {
			normalized[i] = line
			continue
		}

		normalized[i] = normalizeTextLine(line)
	}

	return strings.Join(normalized, "\n")
}

func normalizeTextLine(line string) string {
	result := line
	for _, transform := range textLineTransforms {
		result = transform(result)
	}
	return result
}

func detectFenceMarker(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	switch {
	case strings.HasPrefix(trimmed, "```"):
		return "```", true
	case strings.HasPrefix(trimmed, "~~~"):
		return "~~~", true
	default:
		return "", false
	}
}

var chineseSentencePunct = strings.NewReplacer(
	"，", ", ",
	"。", ", ",
	"；", ", ",
	"、", ", ",
)

var chineseOtherPunct = strings.NewReplacer(
	"：", ": ",
	"！", "! ",
	"？", "? ",
)

var chineseBrackets = strings.NewReplacer(
	"（", "(",
	"）", ")",
	"【", "[",
	"】", "]",
	"\u201C", "\"",
	"\u201D", "\"",
	"\u2018", "'",
	"\u2019", "'",
)

func replaceChinesePunctuation(line string) string {
	line = chineseSentencePunct.Replace(line)
	line = chineseOtherPunct.Replace(line)
	line = chineseBrackets.Replace(line)
	return line
}

var reCommaNoSpace = regexp.MustCompile(`,(?:[^ \n]|$)`)

func normalizeCommaSpacing(line string) string {
	return reCommaNoSpace.ReplaceAllStringFunc(line, func(match string) string {
		if len(match) == 1 {
			return ", "
		}
		return ", " + match[1:]
	})
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		(r >= 0x3040 && r <= 0x30FF) ||
		(r >= 0xAC00 && r <= 0xD7AF)
}

func isLatinOrDigit(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

func isOpenBracket(r rune) bool {
	switch r {
	case '(', '[', '{', '<':
		return true
	}
	return false
}

func isCloseBracket(r rune) bool {
	switch r {
	case ')', ']', '}', '>':
		return true
	}
	return false
}

func shouldInsertBoundarySpace(left, right rune) bool {
	if isCJK(left) && (isLatinOrDigit(right) || isOpenBracket(right)) {
		return true
	}
	if (isLatinOrDigit(left) || isCloseBracket(left)) && isCJK(right) {
		return true
	}
	return false
}

func insertCJKLatinSpaces(line string) string {
	runes := []rune(line)
	var b strings.Builder
	b.Grow(len(line) + 8)

	for i, current := range runes {
		b.WriteRune(current)
		if i+1 >= len(runes) {
			break
		}
		next := runes[i+1]
		if shouldInsertBoundarySpace(current, next) {
			b.WriteRune(' ')
		}
	}

	return b.String()
}

var markdownProtectedSpan = regexp.MustCompile(`!?\[[^\]]*\]\([^)]+\)|https?://[^\s<>)\]]+`)

func insertCJKLatinSpacesSafe(line string) string {
	protectedRanges := markdownProtectedSpan.FindAllStringIndex(line, -1)
	if len(protectedRanges) == 0 {
		return insertCJKLatinSpaces(line)
	}

	var b strings.Builder
	lastEnd := 0

	for _, protectedRange := range protectedRanges {
		start, end := protectedRange[0], protectedRange[1]

		if start > lastEnd {
			b.WriteString(insertCJKLatinSpaces(line[lastEnd:start]))
		}

		if start > 0 {
			left, _ := utf8LastRuneInString(line[:start])
			right, _ := utf8.DecodeRuneInString(line[start:end])
			if left != utf8.RuneError && right != utf8.RuneError &&
				shouldInsertBoundarySpace(left, right) &&
				!builderEndsWithSpace(&b) {
				b.WriteRune(' ')
			}
		}

		b.WriteString(line[start:end])
		lastEnd = end
	}

	if lastEnd < len(line) {
		left, _ := utf8LastRuneInString(line[:lastEnd])
		right, _ := utf8.DecodeRuneInString(line[lastEnd:])
		if lastEnd > 0 &&
			left != utf8.RuneError &&
			right != utf8.RuneError &&
			shouldInsertBoundarySpace(left, right) &&
			!strings.HasPrefix(line[lastEnd:], " ") {
			b.WriteRune(' ')
		}
		b.WriteString(insertCJKLatinSpaces(line[lastEnd:]))
	}

	return b.String()
}

func utf8LastRuneInString(s string) (rune, int) {
	return utf8.DecodeLastRuneInString(s)
}

func builderEndsWithSpace(b *strings.Builder) bool {
	return strings.HasSuffix(b.String(), " ")
}

var reTrailingPeriodAndComma = regexp.MustCompile(`[.,]\s*$`)

func stripTrailingPeriodAndComma(line string) string {
	return reTrailingPeriodAndComma.ReplaceAllString(line, "")
}
