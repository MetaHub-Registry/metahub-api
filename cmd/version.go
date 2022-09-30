package cmd

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func Version(cCtx *cli.Context) error {
	fmt.Println("v0.2.0")
	return nil
}
