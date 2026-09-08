package mdparse

import (
	"bytes"
	"html"
	"net/url"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldtext "github.com/yuin/goldmark/text"
)

type Link struct {
	Destination string
	Start       int
	End         int
	Line        int
	Column      int
	reference   bool
}

type Document struct {
	Links   []Link
	Anchors map[string]struct{}
}

type Edit struct {
	Start int
	End   int
	Text  string
}

var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
)

func Parse(source []byte) Document {
	context := parser.NewContext()
	root := markdown.Parser().Parse(goldtext.NewReader(source), parser.WithContext(context))
	excluded := make([]bool, len(source))
	markRawBlocks(root, excluded)
	markFencedBlocks(source, excluded)
	markCodeSpans(source, excluded)

	doc := Document{Anchors: map[string]struct{}{}}
	usedAnchors := map[string]struct{}{}
	_ = ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if heading, ok := node.(*ast.Heading); ok {
			base := slug(headingPlainText(heading, source))
			anchor := base
			for suffix := 1; ; suffix++ {
				if _, exists := usedAnchors[anchor]; !exists {
					break
				}
				anchor = base + "-" + itoa(suffix)
			}
			usedAnchors[anchor] = struct{}{}
			doc.Anchors[anchor] = struct{}{}
		}
		return ast.WalkContinue, nil
	})

	doc.Links = semanticLinks(root, context, scanLinks(source, excluded))
	return doc
}

func semanticLinks(root ast.Node, context parser.Context, candidates []Link) []Link {
	inline := map[string]int{}
	references := map[string]int{}
	_ = ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *ast.Link:
			inline[DecodeDestination(string(n.Destination))]++
		case *ast.Image:
			inline[DecodeDestination(string(n.Destination))]++
		case *ast.LinkReferenceDefinition:
			references[DecodeDestination(string(n.Destination))]++
		}
		return ast.WalkContinue, nil
	})
	if len(references) == 0 {
		for _, reference := range context.References() {
			references[DecodeDestination(string(reference.Destination()))]++
		}
	}
	result := make([]Link, 0, len(candidates))
	for _, candidate := range candidates {
		destination := DecodeDestination(candidate.Destination)
		pool := inline
		if candidate.reference {
			pool = references
		}
		if pool[destination] == 0 {
			continue
		}
		pool[destination]--
		result = append(result, candidate)
	}
	return result
}

func Apply(source []byte, edits []Edit) []byte {
	sort.Slice(edits, func(i, j int) bool { return edits[i].Start > edits[j].Start })
	result := append([]byte(nil), source...)
	for _, edit := range edits {
		next := make([]byte, 0, len(result)-(edit.End-edit.Start)+len(edit.Text))
		next = append(next, result[:edit.Start]...)
		next = append(next, edit.Text...)
		next = append(next, result[edit.End:]...)
		result = next
	}
	return result
}

func DecodeDestination(raw string) string {
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\\' && i+1 < len(raw) && strings.ContainsRune("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", rune(raw[i+1])) {
			i++
			b.WriteByte(raw[i])
			continue
		}
		b.WriteByte(raw[i])
	}
	return html.UnescapeString(b.String())
}

func EncodePath(path string) string {
	parts := strings.Split(filepathSlash(path), "/")
	for i, part := range parts {
		decoded, err := url.PathUnescape(part)
		if err == nil {
			part = decoded
		}
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func SplitDestination(raw string) (path, suffix string) {
	cut := len(raw)
	if i := strings.IndexByte(raw, '?'); i >= 0 && i < cut {
		cut = i
	}
	if i := strings.IndexByte(raw, '#'); i >= 0 && i < cut {
		cut = i
	}
	return raw[:cut], raw[cut:]
}

func Fragment(raw string) (string, bool) {
	i := strings.IndexByte(raw, '#')
	if i < 0 {
		return "", false
	}
	fragment := raw[i+1:]
	decoded, err := url.PathUnescape(fragment)
	if err != nil {
		return fragment, true
	}
	return decoded, true
}

func IsLocal(raw string) bool {
	raw = DecodeDestination(raw)
	path, _ := SplitDestination(raw)
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		return false
	}
	if i := strings.IndexByte(path, ':'); i > 0 {
		for _, r := range path[:i] {
			if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '+' || r == '-' || r == '.') {
				return true
			}
		}
		return false
	}
	return true
}

func DecodePath(raw string) (string, error) {
	raw = DecodeDestination(raw)
	path, _ := SplitDestination(raw)
	return url.PathUnescape(path)
}

func markRawBlocks(root ast.Node, excluded []bool) {
	_ = ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *ast.CodeBlock:
			markLines(n.Lines(), excluded)
		case *ast.FencedCodeBlock:
			markLines(n.Lines(), excluded)
		case *ast.HTMLBlock:
			markLines(n.Lines(), excluded)
			if n.HasClosure() {
				mark(excluded, n.ClosureLine.Start, n.ClosureLine.Stop)
			}
		case *ast.RawHTML:
			for i := 0; i < n.Segments.Len(); i++ {
				segment := n.Segments.At(i)
				mark(excluded, segment.Start, segment.Stop)
			}
		}
		return ast.WalkContinue, nil
	})
}

func markLines(lines *goldtext.Segments, excluded []bool) {
	for i := 0; i < lines.Len(); i++ {
		segment := lines.At(i)
		mark(excluded, segment.Start, segment.Stop)
	}
}

func mark(excluded []bool, start, end int) {
	if start < 0 {
		start = 0
	}
	if end > len(excluded) {
		end = len(excluded)
	}
	for i := start; i < end; i++ {
		excluded[i] = true
	}
}

func markCodeSpans(source []byte, excluded []bool) {
	for i := 0; i < len(source); {
		if excluded[i] || source[i] != '`' || escaped(source, i) {
			i++
			continue
		}
		run := byteRun(source, i, '`')
		close := findRun(source, i+run, '`', run, excluded)
		if close < 0 {
			i += run
			continue
		}
		mark(excluded, i, close+run)
		i = close + run
	}
}

func markFencedBlocks(source []byte, excluded []bool) {
	inFence := false
	var marker byte
	markerLength := 0
	for start := 0; start <= len(source); {
		end := bytes.IndexByte(source[start:], '\n')
		if end < 0 {
			end = len(source)
		} else {
			end += start
		}
		i := start
		spaces := 0
		for i < end && source[i] == ' ' && spaces < 4 {
			i++
			spaces++
		}
		isMarker := spaces <= 3 && i < end && (source[i] == '`' || source[i] == '~')
		run := 0
		if isMarker {
			run = byteRun(source, i, source[i])
		}
		validOpening := isMarker && run >= 3 && (source[i] != '`' || !bytes.ContainsRune(source[i+run:end], '`'))
		if !inFence && validOpening {
			inFence = true
			marker = source[i]
			markerLength = run
			mark(excluded, start, min(end+1, len(source)))
		} else if inFence {
			mark(excluded, start, min(end+1, len(source)))
			if isMarker && source[i] == marker {
				if run >= markerLength && onlySpace(source[i+run:end]) {
					inFence = false
				}
			}
		}
		if end == len(source) {
			break
		}
		start = end + 1
	}
}

func scanLinks(source []byte, excluded []bool) []Link {
	var links []Link
	lineStart := 0
	for lineStart <= len(source) {
		lineEnd := bytes.IndexByte(source[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(source)
		} else {
			lineEnd += lineStart
		}
		if start, end, ok := referenceDestination(source, lineStart, lineEnd, excluded); ok {
			link := newLink(source, start, end)
			link.reference = true
			links = append(links, link)
		}
		if lineEnd == len(source) {
			break
		}
		lineStart = lineEnd + 1
	}

	for i := 0; i < len(source); i++ {
		if excluded[i] || source[i] != '[' || escaped(source, i) {
			continue
		}
		close := matchingBracket(source, i, excluded)
		if close < 0 || close+1 >= len(source) || source[close+1] != '(' {
			continue
		}
		start, end, ok := inlineDestination(source, close+2, excluded)
		if ok {
			links = append(links, newLink(source, start, end))
		}
	}
	sort.Slice(links, func(i, j int) bool { return links[i].Start < links[j].Start })
	return links
}

func referenceDestination(source []byte, lineStart, lineEnd int, excluded []bool) (int, int, bool) {
	i, ok := referenceLineStart(source, lineStart, lineEnd)
	if !ok || i >= lineEnd || excluded[i] || source[i] != '[' {
		return 0, 0, false
	}
	close := findUnescaped(source, i+1, lineEnd, ']')
	if close < 0 || close+1 >= lineEnd || source[close+1] != ':' {
		return 0, 0, false
	}
	i = close + 2
	for i < lineEnd && (source[i] == ' ' || source[i] == '\t') {
		i++
	}
	if i == lineEnd && lineEnd < len(source) {
		nextStart := lineEnd + 1
		nextEnd := bytes.IndexByte(source[nextStart:], '\n')
		if nextEnd < 0 {
			nextEnd = len(source)
		} else {
			nextEnd += nextStart
		}
		i, ok = referenceLineStart(source, nextStart, nextEnd)
		if !ok || i == nextEnd || excluded[i] {
			return 0, 0, false
		}
		lineEnd = nextEnd
	}
	return destinationToken(source, i, lineEnd, true)
}

func referenceLineStart(source []byte, start, end int) (int, bool) {
	i := start
	for {
		spaces := 0
		for i < end && source[i] == ' ' && spaces < 4 {
			i++
			spaces++
		}
		if spaces > 3 {
			return 0, false
		}
		if i < end && source[i] == '>' {
			i++
			if i < end && (source[i] == ' ' || source[i] == '\t') {
				i++
			}
			continue
		}
		if markerEnd := listMarkerEnd(source, i, end); markerEnd >= 0 {
			i = markerEnd
			continue
		}
		return i, true
	}
}

func listMarkerEnd(source []byte, start, end int) int {
	if start >= end {
		return -1
	}
	i := start
	if source[i] == '-' || source[i] == '+' || source[i] == '*' {
		i++
	} else {
		digits := 0
		for i < end && source[i] >= '0' && source[i] <= '9' && digits < 10 {
			i++
			digits++
		}
		if digits == 0 || digits > 9 || i >= end || (source[i] != '.' && source[i] != ')') {
			return -1
		}
		i++
	}
	spaces := 0
	for i < end && (source[i] == ' ' || source[i] == '\t') && spaces < 4 {
		i++
		spaces++
	}
	if spaces == 0 {
		return -1
	}
	return i
}

func inlineDestination(source []byte, start int, excluded []bool) (int, int, bool) {
	i := start
	for i < len(source) && (source[i] == ' ' || source[i] == '\t' || source[i] == '\n' || source[i] == '\r') {
		i++
	}
	if i >= len(source) || excluded[i] {
		return 0, 0, false
	}
	return destinationToken(source, i, len(source), false)
}

func destinationToken(source []byte, start, limit int, definition bool) (int, int, bool) {
	if start > limit {
		return 0, 0, false
	}
	if start == limit {
		return start, start, definition
	}
	if source[start] == '<' {
		end := findUnescaped(source, start+1, limit, '>')
		if end < 0 {
			return 0, 0, false
		}
		return start + 1, end, true
	}
	depth := 0
	for i := start; i < limit; i++ {
		c := source[i]
		if c == '\\' && i+1 < limit {
			i++
			continue
		}
		if c == '\n' || c == '\r' {
			return 0, 0, false
		}
		switch c {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return start, i, true
			}
			depth--
		case ' ', '\t':
			return start, i, i > start || definition
		}
	}
	if definition {
		return start, limit, true
	}
	return 0, 0, false
}

func matchingBracket(source []byte, start int, excluded []bool) int {
	depth := 1
	for i := start + 1; i < len(source); i++ {
		if excluded[i] || escaped(source, i) {
			continue
		}
		switch source[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func newLink(source []byte, start, end int) Link {
	line := 1 + bytes.Count(source[:start], []byte{'\n'})
	last := bytes.LastIndexByte(source[:start], '\n')
	column := utf8.RuneCount(source[last+1:start]) + 1
	return Link{Destination: string(source[start:end]), Start: start, End: end, Line: line, Column: column}
}

func slug(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r), unicode.IsMark(r), unicode.Is(unicode.Pc, r), r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}

func headingPlainText(heading *ast.Heading, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(heading, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || node == heading {
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *ast.RawHTML:
			return ast.WalkSkipChildren, nil
		case *ast.CodeSpan:
			for child := n.FirstChild(); child != nil; child = child.NextSibling() {
				value := child.(*ast.Text).Segment.Value(source)
				if bytes.HasSuffix(value, []byte{'\n'}) {
					b.Write(value[:len(value)-1])
					b.WriteByte(' ')
				} else {
					b.Write(value)
				}
			}
			return ast.WalkSkipChildren, nil
		case *ast.Text:
			value := string(n.Text(source))
			if !n.IsRaw() {
				value = DecodeDestination(value)
			}
			b.WriteString(value)
			return ast.WalkSkipChildren, nil
		case *ast.String:
			value := string(n.Text(source))
			if !n.IsRaw() && !n.IsCode() {
				value = DecodeDestination(value)
			}
			b.WriteString(value)
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

func escaped(source []byte, pos int) bool {
	count := 0
	for pos > 0 && source[pos-1] == '\\' {
		count++
		pos--
	}
	return count%2 == 1
}

func findUnescaped(source []byte, start, end int, needle byte) int {
	for i := start; i < end; i++ {
		if source[i] == needle && !escaped(source, i) {
			return i
		}
	}
	return -1
}

func byteRun(source []byte, start int, value byte) int {
	i := start
	for i < len(source) && source[i] == value {
		i++
	}
	return i - start
}

func findRun(source []byte, start int, value byte, length int, excluded []bool) int {
	for i := start; i < len(source); i++ {
		if excluded[i] || source[i] != value {
			continue
		}
		if byteRun(source, i, value) == length {
			return i
		}
	}
	return -1
}

func filepathSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}

func onlySpace(value []byte) bool {
	for _, c := range value {
		if c != ' ' && c != '\t' && c != '\r' {
			return false
		}
	}
	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
