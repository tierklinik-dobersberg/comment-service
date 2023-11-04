package mentions

import (
	"context"

	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

type Resolver interface {
	ResolveMention(*Node) (profile *idmv1.Profile, err error)
}

type ResolverFunc func(*Node) (*idmv1.Profile, error)

func (fn ResolverFunc) ResolveMention(n *Node) (*idmv1.Profile, error) {
	return fn(n)
}

type Extender struct {
	Context  context.Context
	Resolver Resolver
}

func (e *Extender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(
			util.Prioritized(
				&Parser{
					Context:  e.Context,
					Resolver: e.Resolver,
				},
				999,
			),
		),
	)

	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(
				&Renderer{
					Context: e.Context,
				},
				999,
			),
		),
	)
}

var _ goldmark.Extender = (*Extender)(nil)
