package cmds

import (
	"strings"

	"github.com/bufbuild/connect-go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	commentv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/comment/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
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
		DeleteScopeCommand(root),
	)

	return cmd
}

func CreateScopeCommand(root *cli.Root) *cobra.Command {
	var (
		viewTemplate string
		notifyType   string
		id           string
	)

	cmd := &cobra.Command{
		Use:  "create",
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cli := root.Comments()

			if id == "" {
				id = strings.ToLower(args[0])
			}

			var notifyTypePb commentv1.NotificationType
			switch notifyType {
			case "":
				notifyTypePb = commentv1.NotificationType_NOTIFICATION_TYPE_UNSPECIFIED
			case "sms":
				notifyTypePb = commentv1.NotificationType_NOTIFICATION_TYPE_SMS
			case "email", "mail":
				notifyTypePb = commentv1.NotificationType_NOTIFICATION_TYPE_MAIL
			default:
				logrus.Fatalf("invalid value for --notify-type")
			}

			res, err := cli.CreateScope(root.Context(), connect.NewRequest(&commentv1.CreateScopeRequest{
				Name:                   args[0],
				ViewCommentUrlTemplate: viewTemplate,
				NotifcationType:        notifyTypePb,
				Id:                     id,
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
