package cmds

import (
	"strings"

	"github.com/bufbuild/connect-go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	commentv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/comment/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func ScopeCommand(root *cli.Root) *cobra.Command {
	cmd := &cobra.Command{
		Use: "comment-scopes",
		Run: func(_ *cobra.Command, args []string) {
			cli := root.Comments()

			scopeListResponse, err := cli.ListScope(root.Context(), connect.NewRequest(&commentv1.ListScopeRequest{}))
			if err != nil {
				logrus.Fatal(err)
			}

			root.Print(scopeListResponse.Msg)
		},
	}

	cmd.AddCommand(
		CreateScopeCommand(root),
		UpdateScopeCommand(root),
		DeleteScopeCommand(root),
	)

	return cmd
}

func CreateScopeCommand(root *cli.Root) *cobra.Command {
	var (
		viewTemplate string
		notifyType   string
		id           string
		owners       []string
	)

	cmd := &cobra.Command{
		Use:  "create",
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cli := root.Comments()

			if id == "" {
				id = strings.ToLower(args[0])
			}

			owners, err := root.ResolveUserIds(root.Context(), owners)
			if err != nil {
				logrus.Fatalf("failed to resolve owner ids: %s", err)
			}

			res, err := cli.CreateScope(root.Context(), connect.NewRequest(&commentv1.CreateScopeRequest{
				Name:                   args[0],
				ViewCommentUrlTemplate: viewTemplate,
				NotifcationType:        notifyTypePb(notifyType),
				Id:                     id,
				ScopeOwnerIds:          owners,
			}))
			if err != nil {
				logrus.Fatal(err)
			}

			root.Print(res.Msg)
		},
	}

	f := cmd.Flags()
	{
		f.StringVar(&viewTemplate, "view-tmpl", "", "The go text/template string to construct a view-comment URL for this scope")
		f.StringVar(&notifyType, "notify-type", "", "The notification type. either unspecified (empty), sms or email)")
		f.StringVar(&id, "id", "", "The ID for the new scope")
	}

	return cmd
}

func UpdateScopeCommand(root *cli.Root) *cobra.Command {
	req := &commentv1.UpdateScopeRequest{
		WriteMask: &fieldmaskpb.FieldMask{},
	}

	var notifyType string

	cmd := &cobra.Command{
		Use:  "update",
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			flagUpdates := [][]string{
				{"name", "name"},
				{"notify-type", "notification_type"},
				{"view-tmpl", "view_comment_url_template"},
				{"add-owner", "add_scope_owner_ids"},
				{"remove-owner", "remove_scope_owner_ids"},
			}

			for _, fs := range flagUpdates {
				if cmd.Flag(fs[0]).Changed {
					req.WriteMask.Paths = append(req.WriteMask.Paths, fs[1])
				}

				if fs[0] == "notify-type" {
					req.NotifcationType = notifyTypePb(notifyType)
				}
			}

			req.Id = args[0]

			var err error
			req.AddScopeOwnerIds, err = root.ResolveUserIds(root.Context(), req.AddScopeOwnerIds)
			if err != nil {
				logrus.Fatalf("failed to resolve owner ids for --add-owner: %s", err)
			}

			req.RemoveScopeOwnerIds, err = root.ResolveUserIds(root.Context(), req.RemoveScopeOwnerIds)
			if err != nil {
				logrus.Fatalf("failed to resolve owner ids for --remove-owner: %s", err)
			}

			res, err := root.Comments().UpdateScope(root.Context(), connect.NewRequest(req))
			if err != nil {
				logrus.Fatalf("failed to update scope: %s", err)
			}

			root.Print(res.Msg)
		},
	}

	f := cmd.Flags()
	{
		f.StringVar(&req.Name, "name", "", "The new name for the scope")
		f.StringVar(&req.ViewCommentUrlTemplate, "view-tmpl", "", "The new view template URL")
		f.StringVar(&notifyType, "notify-type", "", "The new default notification type")
		f.StringSliceVar(&req.AddScopeOwnerIds, "add-owner", nil, "Add user as an scope owner by id or name")
		f.StringSliceVar(&req.RemoveScopeOwnerIds, "remove-owner", nil, "Remove user from scope owners by id or name")
	}

	return cmd
}

func DeleteScopeCommand(root *cli.Root) *cobra.Command {
	return &cobra.Command{
		Use:     "delete",
		Aliases: []string{"remove", "rm"},
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cli := root.Comments()

			res, err := cli.DeleteScope(root.Context(), connect.NewRequest(&commentv1.DeleteScopeRequest{
				Id: args[0],
			}))
			if err != nil {
				logrus.Fatal(err)
			}

			root.Print(res.Msg)
		},
	}
}

func notifyTypePb(nt string) commentv1.NotificationType {
	switch nt {
	case "":
		return commentv1.NotificationType_NOTIFICATION_TYPE_UNSPECIFIED
	case "sms":
		return commentv1.NotificationType_NOTIFICATION_TYPE_SMS
	case "email", "mail":
		return commentv1.NotificationType_NOTIFICATION_TYPE_MAIL
	default:
		logrus.Fatalf("invalid value for --notify-type")
	}

	// not reachable
	return commentv1.NotificationType_NOTIFICATION_TYPE_UNSPECIFIED
}
