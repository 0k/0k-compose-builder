package internal

import (
    "context"
    "fmt"

    "github.com/moby/buildkit/client/llb"
)

type BuildContext struct {
    RunnerImage       string
    ProjectName       string
    CharmStorePath    string
    ConfigStorePath   string
    RelationDataPath  string
    DockerComposePath string
	ComposeCachePath  string
}


// BuildLLB constructs the LLB definition
func BuildLLB(ctx context.Context, bctx *BuildContext) (*llb.Definition, error) {

	runner := llb.Image(bctx.RunnerImage)
	
    relation := llb.Scratch()
    configstore := llb.Scratch()
    dockerCompose := llb.Scratch()
    charmstore := llb.Local("charm-store")

    run := func(configstoreState llb.State, relationState llb.State, cmd llb.RunOption) (llb.State, llb.State) {
        runState := runner.Run(
            cmd,
            llb.AddMount(bctx.CharmStorePath, charmstore),
            llb.AddMount(bctx.ConfigStorePath, configstore),
            llb.AddMount(bctx.DockerComposePath, dockerCompose),
            llb.AddMount(bctx.RelationDataPath, relation),
            llb.AddHostBindMount(bctx.ComposeCachePath, bctx.ComposeCachePath),
        )

        configstoreState = runState.GetMount(bctx.ConfigStorePath)
        relationState = runState.GetMount(bctx.RelationDataPath)

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

