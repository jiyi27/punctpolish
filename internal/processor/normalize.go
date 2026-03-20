package processor

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// stage is a document-level transformation: takes the full text, returns transformed text.
type stage func(string) string

// pipeline defines the ordered sequence of normalization stages applied to every document.
var pipeline = []stage{
	linePass(replaceChinesePunctuation), // stage 1: Chinese punctuation → English equivalents
	linePass(normalizeCommaSpacing),     // stage 2: ensure space after comma
	linePass(insertCJKLatinSpacesSafe),  // stage 3: space at CJK↔Latin/digit boundaries
	linePass(collapseSpaces),            // stage 4: collapse runs of spaces (per line)
	stripParaEndSeparators,              // stage 5: remove trailing punctuation at paragraph ends
}

// NormalizeText runs the input through every pipeline stage in order.
// It is a pure function with no side effects.
func NormalizeText(input string) string {
	result := input
	for _, s := range pipeline {
		result = s(result)
	}
	return result
}

// linePass wraps a line-level function into a document-level stage.
func linePass(fn func(string) string) stage {
	return func(input string) string {
		lines := strings.Split(input, "\n")
		for i, line := range lines {
			lines[i] = fn(line)
		}
		return strings.Join(lines, "\n")
	}
}

// stripParaEndSeparators is a document-level stage that removes trailing
// punctuation from lines that end a sentence: paragraph boundaries (followed
// by a blank line or end of input) and list items.
func stripParaEndSeparators(input string) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		isParaEnd := i == len(lines)-1 || strings.TrimSpace(lines[i+1]) == ""
		if isParaEnd || isListItem(trimmed) {
			lines[i] = reTrailingSep.ReplaceAllString(line, "")
		}
	}
	return strings.Join(lines, "\n")
}

// reListItem matches Markdown list item prefixes: "- ", "* ", "+ ", or "1. ".
var reListItem = regexp.MustCompile(`^[-*+] |^\d+\. `)

func isListItem(trimmed string) bool {
	return reListItem.MatchString(trimmed)
}

// ---------------------------------------------------------------------------
// Line-level transformation functions (used as pipeline stages via linePass)
// ---------------------------------------------------------------------------

// chineseSentencePunct maps Chinese sentence / list punctuation to ", ".
var chineseSentencePunct = strings.NewReplacer(
	"，", ", ",
	"。", ", ",
	"；", ", ",
	"、", ", ",
)

// chineseOtherPunct maps other Chinese punctuation to English equivalents + space.
var chineseOtherPunct = strings.NewReplacer(
	"：", ": ",
	"！", "! ",
	"？", "? ",
)

// chineseBrackets maps Chinese brackets and quotes to ASCII equivalents.
var chineseBrackets = strings.NewReplacer(
	"（", "(",
	"）", ")",
	"【", "[",
	"】", "]",
	"\u201C", "\"", // " left double quotation
	"\u201D", "\"", // " right double quotation
	"\u2018", "'",  // ' left single quotation
	"\u2019", "'",  // ' right single quotation
)

func replaceChinesePunctuation(s string) string {
	s = chineseSentencePunct.Replace(s)
	s = chineseOtherPunct.Replace(s)
	s = chineseBrackets.Replace(s)
	return s
}

// reCommaNoSpace matches a comma not already followed by a space.
var reCommaNoSpace = regexp.MustCompile(`,(?:[^ \n]|$)`)

func normalizeCommaSpacing(s string) string {
	return reCommaNoSpace.ReplaceAllStringFunc(s, func(m string) string {
		if len(m) == 1 {
			return ", "
		}
		return ", " + m[1:]
	})
}

// isCJK reports whether r is a CJK unified ideograph, Hiragana, Katakana, or Hangul.
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

func insertCJKLatinSpaces(s string) string {
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i, r := range runes {
		b.WriteRune(r)
		if i+1 >= len(runes) {
			break
		}
		next := runes[i+1]
		if shouldInsertBoundarySpace(r, next) {
			b.WriteRune(' ')
		}
	}
	return b.String()
}

// markdownProtectedSpan matches inline Markdown links/images and bare HTTP(S)
// URLs so we can avoid changing their contents during spacing insertion.
var markdownProtectedSpan = regexp.MustCompile(`!?\[[^\]]*\]\([^)]+\)|https?://[^\s<>)\]]+`)

func insertCJKLatinSpacesSafe(s string) string {
	matches := markdownProtectedSpan.FindAllStringIndex(s, -1)
	if len(matches) == 0 {
		return insertCJKLatinSpaces(s)
	}

	var b strings.Builder
	last := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		if start > last {
			b.WriteString(insertCJKLatinSpaces(s[last:start]))
		}
		if start > 0 {
			left, _ := utf8LastRuneInString(s[:start])
			right, _ := utf8.DecodeRuneInString(s[start:end])
			if left != utf8.RuneError && right != utf8.RuneError && shouldInsertBoundarySpace(left, right) && !builderEndsWithSpace(&b) {
				b.WriteRune(' ')
			}
		}
		b.WriteString(s[start:end])
		last = end
	}
	if last < len(s) {
		if len(s) > 0 {
			left, _ := utf8LastRuneInString(s[:last])
			right, _ := utf8.DecodeRuneInString(s[last:])
			if left != utf8.RuneError && right != utf8.RuneError && shouldInsertBoundarySpace(left, right) && !strings.HasPrefix(s[last:], " ") {
				b.WriteRune(' ')
			}
		}
		b.WriteString(insertCJKLatinSpaces(s[last:]))
	}
	return b.String()
}

func utf8LastRuneInString(s string) (rune, int) {
	return utf8.DecodeLastRuneInString(s)
}

func builderEndsWithSpace(b *strings.Builder) bool {
	out := b.String()
	return strings.HasSuffix(out, " ")
}

// reMultiSpace matches two or more consecutive spaces (not newlines).
var reMultiSpace = regexp.MustCompile(` {2,}`)

func collapseSpaces(s string) string {
	leadLen := 0
	for leadLen < len(s) && (s[leadLen] == ' ' || s[leadLen] == '\t') {
		leadLen++
	}
	lead := s[:leadLen]
	rest := s[leadLen:]
	return lead + reMultiSpace.ReplaceAllString(rest, " ")
}

// reTrailingSep matches a trailing punctuation character (and any following
// spaces) at the end of a line.
var reTrailingSep = regexp.MustCompile(`[,;:!?]\s*$`)
