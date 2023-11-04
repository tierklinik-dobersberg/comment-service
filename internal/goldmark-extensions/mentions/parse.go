package mentions

import (
	"bytes"
	"context"
	"unicode"
	"unicode/utf8"

	"github.com/tierklinik-dobersberg/apis/pkg/log"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type Parser struct {
	Context context.Context

	Resolver Resolver
}

func (*Parser) Trigger() []byte {
	return []byte{'@'}
}

func (p *Parser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, seg := block.PeekLine()

	ctx := p.Context
	if ctx == nil {
		ctx = context.Background()
	}

	l := log.L(ctx)

	l.Debugf("parsing line %q", string(line))

	if len(line) == 0 || line[0] != '@' {
		l.Debugf("line is empty or does not start with a mention symbol")

		return nil
	}

	line = line[1:]

	end := getSpan(line)
	if end < 0 {
		l.Errorf("failed to get end of span")
		return nil
	}

	seg = seg.WithStop(seg.Start + end + 1) // + '@'

	n := Node{
		Tag: block.Value(seg.WithStart(seg.Start + 1)),
	}

	if res := p.Resolver; res != nil {
		profile, err := res.ResolveMention(&n)
		if err != nil {
			l.Errorf("failed to resolve profile %q: %s", n.Tag, err)

			return nil
		}

		n.Profile = profile
	}

	// do not append the next segment as as text because
	// we render the displayName of the user in "Renderer" instead of
	// the specified ID/username:
	//
	// -- n.AppendChild(&n, ast.NewTextSegment(seg))

	block.Advance(seg.Len())

	return &n
}

func getSpan(line []byte) int {
	start, sz := utf8.DecodeRune(line)

	if unicode.IsLetter(start) || unicode.IsNumber(start) {
		// skip the first symbol
		line = line[sz:]

		if i := bytes.IndexFunc(line, endOfMention); i >= 0 {
			return i + sz
		}

		return len(line) + sz
	}

	// invalid mention tag; must either start with a letter or a number
	return -1
}

func endOfMention(r rune) bool {
	return !unicode.IsNumber(r) && !unicode.IsLetter(r) && r != '-'
}

var _ parser.InlineParser = (*Parser)(nil)
