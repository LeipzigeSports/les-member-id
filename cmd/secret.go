// Package cmd provides functions for creating a CLI application to run the member ID service.
package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/LeipzigeSports/les-member-id/internal/secure"
	"github.com/urfave/cli/v3"
)

// ErrSecretCommand is a generic error that may be returned during execution of the secret command.
var ErrSecretCommand = errors.New("error during secret command execution")

// BuildSecretCommand creates a sub-command for generating cryptograhically strong secret valus.
//
//nolint:forbidigo
func BuildSecretCommand() *cli.Command {
	return &cli.Command{
		Name:  "secret",
		Usage: "generate a cryptographically strong secret",
		Action: func(_ context.Context, _ *cli.Command) error {
			gen := secure.NewCSPRNGStateGenerator()

			secret, err := gen.GetState()
			if err != nil {
				return fmt.Errorf("%w: %w", ErrSecretCommand, err)
			}

			fmt.Println(secret)

			return nil
		},
	}
}
