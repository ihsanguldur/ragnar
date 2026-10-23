package chunk

import (
	"strings"
	"unicode/utf8"

	"github.com/ihsanguldur/ragnar/internal/extract"
)

// Chunk is a piece of a document small enough to embed and to show the model
// as one source. A chunk can cross page boundaries; PageStart and PageEnd are
// the first and last pages it contains text from.
type Chunk struct {
	Index     int
	PageStart int
	PageEnd   int
	Content   string
}

// piece is a sentence, or part of an overlong sentence, with the page it
// came from. newPara marks the first piece of a paragraph, so paragraph
// breaks survive when pieces are joined back together.
type piece struct {
	page    int
	text    string
	newPara bool
}

// Split cuts pages into chunks of about size characters, each starting with
// up to overlap characters copied from the end of the previous chunk.
//
// Text is split into sentences first and chunks are built from whole
// sentences, so a chunk never ends mid-sentence unless a single sentence is
// longer than size. The overlap keeps a fact that sits on a chunk boundary
// readable in at least one chunk. Lengths are counted in runes (characters),
// not bytes, so text with multi-byte letters gets the same chunk sizes.
func Split(pages []extract.Page, size, overlap int) []Chunk {
	// Flatten all pages into one list of sentences, so chunks can continue
	// across page breaks.
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
		// The next sentence does not fit: close the chunk and start the next
		// one from its last sentences.
		if curLen > 0 && curLen+n > size {
			chunks = append(chunks, build(len(chunks), cur))
			cur, curLen = tail(cur, overlap)
			// Drop the overlap if keeping it would leave no room for the
			// sentence itself.
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

// sentences splits a paragraph after words ending in '.', '!' or '?'. A
// sentence longer than max is cut at word boundaries. Splitting on whitespace
// also joins the hard line breaks pdftotext puts inside paragraphs. The rule
// is simple on purpose: abbreviations such as "e.g." also end a sentence,
// which only makes some pieces shorter.
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

// tail returns the longest run of whole pieces at the end of pieces that
// fits in overlap characters, and its length. The result is a copy, so it
// does not share memory with the slice of the previous chunk.
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

// build joins pieces into a chunk: a blank line between paragraphs, a space
// between sentences of the same paragraph.
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
