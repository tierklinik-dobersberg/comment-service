package cmds

import (
	"io"
	"os"
	"strings"

	"github.com/bufbuild/connect-go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	commentv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/comment/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
)

func CommentsCommand(root *cli.Root) *cobra.Command {
	var (
		recurse   bool
		byScope   bool
		render    bool
		reference string
	)

	cmd := &cobra.Command{
		Use:     "comments",
		Aliases: []string{"comment"},
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cli := root.Comments()

			if byScope {
				res, err := cli.ListComments(root.Context(), connect.NewRequest(&commentv1.ListCommentsRequest{
					Scope:      args[0],
					Recurse:    recurse,
					RenderHtml: render,
					Reference:  reference,
				}))
				if err != nil {
					logrus.Fatalf("failed to load comment: %s", err)
				}
				root.Print(res.Msg)

				return
			}

			res, err := cli.GetComment(root.Context(), connect.NewRequest(&commentv1.GetCommentRequest{
				Id:      args[0],
				Recurse: recurse,
			}))
			if err != nil {
				logrus.Fatalf("failed to load comment: %s", err)
			}
			root.Print(res.Msg)
		},
	}

	cmd.Flags().BoolVar(&recurse, "recurse", false, "Include all answers as well")
	cmd.Flags().BoolVar(&render, "render-html", false, "Render comment content as HTML")
	cmd.Flags().BoolVar(&byScope, "by-scope", false, "Get all answers by scope")
	cmd.Flags().StringVar(&reference, "ref", "", "Filter comments by reference")

	cmd.AddCommand(
		CreateCommentCommand(root),
	)

	return cmd
}

func CreateCommentCommand(root *cli.Root) *cobra.Command {
	var (
		content   string
		parent    string
		scope     string
		reference string
	)

	cmd := &cobra.Command{
		Use: "create",
		Run: func(cmd *cobra.Command, args []string) {
			cli := root.Comments()

			if scope == "" {
				if parent == "" {
					logrus.Fatalf("scope must be set if parent is not")
				}

				parentComment, err := cli.GetComment(root.Context(), connect.NewRequest(&commentv1.GetCommentRequest{
					Id:      parent,
					Recurse: false,
				}))
				if err != nil {
					logrus.Fatalf("failed to load parent comment: %s", err)
				}

				scope = parentComment.Msg.Result.Comment.Scope
			}

			if strings.HasPrefix(content, "@") {
				filename := strings.TrimPrefix(content, "@")

				var (
					blob []byte
					err  error
				)

				switch filename {
				case "-":
					blob, err = io.ReadAll(os.Stdin)
				default:
					blob, err = os.ReadFile(filename)
				}

				if err != nil {
					logrus.Fatalf("failed to read content from %q: %s", filename, err)
				}

				content = string(blob)
			}

			req := &commentv1.CreateCommentRequest{
				Content: content,
			}

			if parent != "" {
				req.Kind = &commentv1.CreateCommentRequest_ParentId{
					ParentId: parent,
				}
			} else {
				req.Kind = &commentv1.CreateCommentRequest_Root{
					Root: &commentv1.RootComment{
						Scope:     scope,
						Reference: reference,
					},
				}
			}

			res, err := cli.CreateComment(root.Context(), connect.NewRequest(req))
			if err != nil {
				logrus.Fatalf("failed to create comment: %s", err)
			}

			root.Print(res.Msg)
		},
	}

	f := cmd.Flags()
	{
		f.StringVar(&content, "content", "", "The content of the comment or the name of a file prefixed with @")
		f.StringVar(&parent, "reply-to", "", "The ID of the comment to which this is a reply")
		f.StringVar(&scope, "scope", "", "The ID of the scope")
		f.StringVar(&reference, "ref", "", "An opaque application specific reference")
	}

	cmd.MarkFlagsMutuallyExclusive("scope", "reply-to")
	_ = cmd.MarkFlagRequired("content")

	return cmd
}
