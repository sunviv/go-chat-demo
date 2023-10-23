package server

import (
	"context"

	"github.com/spf13/cobra"
)

type ServerStartOption struct {
	id     string
	listen string
}

func NewServerStartCmd(ctx context.Context, version string) *cobra.Command {
	opts := &ServerStartOption{}
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Starts a chat server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunServer(ctx, opts, version)
		},
	}
	cmd.PersistentFlags().StringVarP(&opts.id, "serverid", "i", "demo", "server id")
	cmd.PersistentFlags().StringVarP(&opts.listen, "listen", "l", ":8000", "listen address")
	return cmd
}

func RunServer(ctx context.Context, opts *ServerStartOption, version string) error {
	server := NewServer(opts.id, opts.listen)
	defer server.Shutdown()
	return server.Start()
}
