package main

import (
	"context"
	"flag"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/sunviv/go-chat-demo/server"
)

const version = "v1"

func main() {
	flag.Parse()

	root := &cobra.Command{
		Use:     "chat",
		Version: version,
		Short:   "chat demo",
	}
	ctx := context.Background()

	root.AddCommand(server.NewServerStartCmd(ctx, version))
	if err := root.Execute(); err != nil {
		logrus.WithError(err).Fatal("could not run command")
	}
}
