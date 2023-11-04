package main

import (
	"github.com/sirupsen/logrus"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
	"github.com/tierklinik-dobersberg/comment-service/cmds/client/cmds"
)

func main() {
	root := cli.New("commentctl")

	root.AddCommand(
		cmds.ScopeCommand(root),
		cmds.CommentsCommand(root),
	)

	if err := root.Execute(); err != nil {
		logrus.Fatal(err)
	}
}
