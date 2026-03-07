// Package main exposes the command-line functionality and is the main entrypoint for the member ID service.
package main

import (
	"context"
	"log"
	"os"

	"github.com/LeipzigeSports/les-member-id/cmd"
	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:        "member-id",
		Description: "service for issuing member IDs",
		Commands: []*cli.Command{
			cmd.BuildServerCommand(),
			cmd.BuildSecretCommand(),
		},
	}

	err := cmd.Run(context.Background(), os.Args)
	if err != nil {
		log.Fatalf("error in main application: %v", err)
	}
}
