package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"

    "github.com/moby/buildkit/client/llb"
    "github.com/moby/buildkit/solver/pb"
    digest "github.com/opencontainers/go-digest"
    cli "github.com/urfave/cli"
)

// llbOp represents an operation in the LLB definition
type llbOp struct {
	Op         pb.Op
	Digest     digest.Digest
	OpMetadata pb.OpMetadata
}

// loadLLB parses the LLB definition into a slice of llbOp
func loadLLB(def *llb.Definition) ([]llbOp, error) {
    var ops []llbOp
    for _, dt := range def.Def {
        var op pb.Op
        if err := (&op).Unmarshal(dt); err != nil {
            return nil, fmt.Errorf("failed to parse op: %w", err)
        }
        dgst := digest.FromBytes(dt)
		ent := llbOp{Op: op, Digest: dgst, OpMetadata: def.Metadata[dgst]}
        ops = append(ops, ent)
    }
    return ops, nil
}

// dumpLLB outputs the LLB definition in the specified format
func dumpLLB(c *cli.Context, format string, def *llb.Definition) error {
    if format == "llb" {
        return llb.WriteTo(def, os.Stdout)
    } else if format == "json" {
        ops, err := loadLLB(def)
        if err != nil {
            return err
        }
        enc := json.NewEncoder(os.Stdout)
        for _, op := range ops {
            if err := enc.Encode(op); err != nil {
                return err
            }
        }
    } else {
        return fmt.Errorf("unknown format: %s", format)
    }
    return nil
}

// buildLLB constructs the LLB definition
func buildLLB(ctx context.Context) (*llb.Definition, error) {
    runner := llb.Image(getEnv("COMPOSE_DOCKER_IMAGE", "docker.0k.io/compose:latest"))

    projectName := os.Getenv("PROJECT_NAME")
    if projectName == "" {
        log.Fatalf("PROJECT_NAME environment variable is required")
    }

    charmStorePath := getEnv("CHARM_STORE", "/srv/charm-store")
    if _, err := os.Stat(charmStorePath); os.IsNotExist(err) {
        log.Fatalf("CHARM_STORE path %s does not exist", charmStorePath)
    }

    configStorePath := getEnv("CONFIGSTORE", "/srv/datastore/config")
    if _, err := os.Stat(configStorePath); os.IsNotExist(err) {
        log.Fatalf("CONFIGSTORE path %s does not exist", configStorePath)
    }

    relationDataPath := getEnv("RELATION_DATA", "/var/lib/compose/relations")
    dockerComposePath := getEnv("DOCKER_COMPOSE_FRAGMENTS", "/var/lib/compose/docker-compose-fragments")

    relation := llb.Scratch()
    configstore := llb.Scratch()
    dockerCompose := llb.Scratch()
    charmstore := llb.Local("charm-store")

    run := func(configstoreState llb.State, relationState llb.State, cmd llb.RunOption) (llb.State, llb.State) {
        runState := runner.Run(
            cmd,
            llb.AddMount(charmStorePath, charmstore),
            llb.AddMount(configStorePath, configstore),
            llb.AddMount(dockerComposePath, dockerCompose),
            llb.AddMount(relationDataPath, relation),
        )

        configstoreState = runState.GetMount(configStorePath)
        relationState = runState.GetMount(relationDataPath)

        return configstoreState, relationState
    }

    configstore, relation = run(configstore, relation,
        llb.Shlex("touch $CONFIGSTORE/a"),
    )

    finalState := llb.Merge([]llb.State{configstore, relation})

    def, err := finalState.Marshal(ctx, llb.LinuxAmd64)
    if err != nil {
        return nil, fmt.Errorf("Failed to marshal LLB definition: %v", err)
    }

    return def, nil
}

// getEnv retrieves the environment variable or returns the default value
func getEnv(key, defaultValue string) string {
    if value, exists := os.LookupEnv(key); exists && value != "" {
        return value
    }
    return defaultValue
}

func main() {
    app := cli.NewApp()
    app.Name = "myprogram"
    app.Usage = "Build program"

    app.Commands = []cli.Command{
        {
            Name:  "dump",
            Usage: "Dump the LLB definition",
            Flags: []cli.Flag{
                cli.StringFlag{
                    Name:  "format, f",
                    Usage: "Output format (llb or json)",
                    Value: "llb",
                },
            },
            Action: func(c *cli.Context) error {
                format := c.String("format")
                ctx := context.Background()
                def, err := buildLLB(ctx)
                if err != nil {
                    return err
                }
                return dumpLLB(c, format, def)
            },
        },
    }

    app.Action = func(c *cli.Context) error {
        ctx := context.Background()
        def, err := buildLLB(ctx)
        if err != nil {
            return err
        }
        err = llb.WriteTo(def, os.Stdout)
        if err != nil {
            log.Fatalf("Failed to write LLB to stdout: %v", err)
        }
        return nil
    }

    err := app.Run(os.Args)
    if err != nil {
        log.Fatal(err)
    }
}
