package main

import (
    "context"
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
                    format := c.String("format")
                    color := c.Bool("color")
                    ctx := context.Background()
                    buildCtx, err := NewBuildContext()
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
