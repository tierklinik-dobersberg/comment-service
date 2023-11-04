package mentions

import (
	"context"
	"fmt"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

type Renderer struct {
	Context context.Context
}

func (r *Renderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(Kind, r.Render)
}

func (r *Renderer) Render(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*Node)
	if !ok {
		return ast.WalkStop, fmt.Errorf("unexpected node %T, expected *Node", node)
	}

	// make sure we stop in case of an error
	// while this seems to make sense, @-user-metions are rare so
	// goldmark will still continue to parse/render the markdown. Though,
	// maybe it will get support for context.Context at some point.
	select {
	case <-r.Context.Done():
		return ast.WalkStop, r.Context.Err()
	default:
	}

	if entering {
		if err := r.enter(w, n); err != nil {
			return ast.WalkStop, err
		} else {
			r.exit(w, n)
		}
	}

	return ast.WalkContinue, nil
}

func (r *Renderer) enter(w util.BufWriter, n *Node) error {
	_, _ = w.WriteString(`<span class="mention" data-user-id="` + string(n.Profile.User.Id) + `">`)

	var displayName string
	if n.Profile != nil {
		if n.Profile.GetUser().GetDisplayName() != "" {
			displayName = n.Profile.User.DisplayName
		} else {
			displayName = n.Profile.User.Username
		}
	}

	if len(displayName) == 0 {
		w.WriteString(string(n.Tag))

		return nil
	}

	_, _ = w.WriteString("@" + string(displayName))

	return nil
}

func (r *Renderer) exit(w util.BufWriter, n *Node) error {
	_, _ = w.WriteString("</span>")
	return nil
}
