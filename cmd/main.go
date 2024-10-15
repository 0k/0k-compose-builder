package main

import (
    "context"
	"fmt"
    "log"
    "os"
    "github.com/urfave/cli/v2"
    "github.com/0k/0k-compose-builder/internal"
    "github.com/0k/0k-compose-builder/cmd/dump"
)

func main() {
    app := &cli.App{
        Name:  "myprogram",
        Usage: "Build program",

        Commands: []*cli.Command{
            {
                Name:  "dump",
                Usage: "Dump the LLB definition",
                Flags: []cli.Flag{
                    &cli.StringFlag{
                        Name:    "format",
                        Aliases: []string{"f"},
                        Usage:   "Output format (llb, json, dot or yaml)",
                        Value:   "llb",
                    },
                    &cli.BoolFlag{
                        Name:  "color",
                        Usage: "Force syntax highlighting",
                    },
                },
                Action: func(c *cli.Context) error {
					// Get the positional argument
                    if c.Args().Len() < 1 {
                        return fmt.Errorf("scriptsPath and statePath arguments are required")
                    }
                    scriptsPath := c.Args().Get(0)
					statePath := c.Args().Get(1)

                    format := c.String("format")
                    color := c.Bool("color")
                    ctx := context.Background()
                    buildCtx, err := NewBuildContext(scriptsPath, statePath)
                    if err != nil {
                        return err
                    }
                    def, err := internal.BuildLLB(ctx, buildCtx)
                    if err != nil {
                        return err
                    }
                    return dump.DumpLLB(format, def, color)
                },
            },
        },

        Action: func(c *cli.Context) error {
            return nil
        },
    }

    err := app.Run(os.Args)
    if err != nil {
        log.Fatal(err)
    }
}
