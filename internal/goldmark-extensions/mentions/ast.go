package mentions

import (
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	"github.com/yuin/goldmark/ast"
)

var Kind = ast.NewNodeKind("Mention")

type Node struct {
	ast.BaseInline

	Tag     []byte
	Profile *idmv1.Profile
}

func (*Node) Kind() ast.NodeKind {
	return Kind
}

func (n *Node) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{
		"ID": string(n.Tag),
	}, nil)
}
