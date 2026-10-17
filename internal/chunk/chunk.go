package chunk

import (
	"strings"
	"unicode/utf8"

	"github.com/ihsanguldur/ragnar/internal/extract"
)

type Chunk struct {
	Index     int
	PageStart int
	PageEnd   int
	Content   string
}

type piece struct {
	page    int
	text    string
	newPara bool
}

func Split(pages []extract.Page, size, overlap int) []Chunk {
	var pieces []piece
	for _, p := range pages {
		for _, para := range strings.Split(p.Text, "\n\n") {
			for i, s := range sentences(para, size) {
				pieces = append(pieces, piece{page: p.Number, text: s, newPara: (i == 0)})
			}
		}
	}

	var chunks []Chunk
	var cur []piece
	curLen := 0
	for _, pc := range pieces {
		n := utf8.RuneCountInString(pc.text)
		if curLen > 0 && curLen+n > size {
			chunks = append(chunks, build(len(chunks), cur))
			cur, curLen = tail(cur, overlap)
			if curLen+n > size {
				cur, curLen = nil, 0
			}
		}
		cur = append(cur, pc)
		curLen += n
	}
	if len(cur) > 0 {
		chunks = append(chunks, build(len(chunks), cur))
	}

	return chunks
}

func sentences(para string, max int) []string {
	var out []string
	var words []string
	n := 0
	for _, w := range strings.Fields(para) {
		wl := utf8.RuneCountInString(w)
		if n > 0 && n+wl > max {
			out = append(out, strings.Join(words, " "))
			words, n = nil, 0
		}
		words = append(words, w)
		n += wl + 1
		if strings.HasSuffix(w, ".") || strings.HasSuffix(w, "!") || strings.HasSuffix(w, "?") {
			out = append(out, strings.Join(words, " "))
			words, n = nil, 0
		}
	}
	if len(words) > 0 {
		out = append(out, strings.Join(words, " "))
	}
	return out
}

func tail(pieces []piece, overlap int) ([]piece, int) {
	total := 0
	i := len(pieces)
	for i > 0 {
		n := utf8.RuneCountInString(pieces[i-1].text)
		if total+n > overlap {
			break
		}
		total += n
		i--
	}
	return append([]piece(nil), pieces[i:]...), total
}

func build(index int, pieces []piece) Chunk {
	var b strings.Builder
	for i, pc := range pieces {
		if i > 0 {
			if pc.newPara {
				b.WriteString("\n\n")
			} else {
				b.WriteString(" ")
			}
		}
		b.WriteString(pc.text)
	}
	return Chunk{
		Index:     index,
		PageStart: pieces[0].page,
		PageEnd:   pieces[len(pieces)-1].page,
		Content:   b.String(),
	}
}
