package internal

import (
	"crypto/sha256"
    "context"
    "fmt"
	"io"
	"io/ioutil"
	"os"
	"log"
	"strings"

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
	ScriptsPath	      string
	StatePath	      string
}


func hashFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func fileExists(filename string) bool {
    _, err := os.Stat(filename)
    if err == nil {
        return true
    }
    if os.IsNotExist(err) {
        return false
    }
    return false
}

type RelationInfo struct {
    State    llb.State
    Name     string
    Services []string
}

// BuildLLB constructs the LLB definition
func BuildLLB(ctx context.Context, bctx *BuildContext) (*llb.Definition, error) {

	runner := llb.Image(bctx.RunnerImage,
		llb.ResolveModePreferLocal,
	)

	var dockerComposeStates []llb.State
	
    relations := llb.Local("relations")
    configStore := llb.Scratch()

    runRelation := func(configStoreState llb.State, relationInfo RelationInfo, cmd llb.RunOption) (llb.State, llb.State) {
		relationDataPath := "/" + bctx.ProjectName + "/" +
			strings.Join(relationInfo.Services, "-") + "/" + relationInfo.Name
        runState := runner.Run(
            cmd,
            llb.AddMount(bctx.ConfigStorePath, configStoreState),
            llb.AddMount(bctx.DockerComposePath, llb.Scratch()),
            llb.AddMount(bctx.RelationDataPath + relationDataPath, relationInfo.State),
            llb.AddHostBindMount("/usr/local/bin/compose-core", "/usr/local/bin/compose-core", llb.Readonly),
            llb.AddHostBindMount("/tmp/statedir", bctx.StatePath),
            llb.AddHostBindMount("/mnt", bctx.ScriptsPath, llb.Readonly),
            llb.AddHostBindMount(bctx.ComposeCachePath, bctx.ComposeCachePath),
            llb.AddHostBindMount(bctx.CharmStorePath, bctx.CharmStorePath, llb.Readonly),
            llb.AddHostBindMount("/var/run/docker.sock", "/var/run/docker.sock"),
        )

        configStoreState = runState.GetMount(bctx.ConfigStorePath)
        relationState := runState.GetMount(bctx.RelationDataPath + relationDataPath)
        dockerComposeState := runState.GetMount(bctx.DockerComposePath)
		dockerComposeStates = append(dockerComposeStates, dockerComposeState)
        return configStoreState, relationState
    }

	runHook := func(configStoreState llb.State, cmd llb.RunOption) (llb.State) {
        runState := runner.Run(
            cmd,
            llb.AddMount(bctx.ConfigStorePath, configStoreState),
            llb.AddMount(bctx.DockerComposePath, llb.Scratch()),
            llb.AddHostBindMount("/usr/local/bin/compose-core", "/usr/local/bin/compose-core", llb.Readonly),
            llb.AddHostBindMount("/tmp/statedir", bctx.StatePath),
            llb.AddHostBindMount("/mnt", bctx.ScriptsPath, llb.Readonly),
            llb.AddHostBindMount(bctx.ComposeCachePath, bctx.ComposeCachePath),
            llb.AddHostBindMount(bctx.CharmStorePath, bctx.CharmStorePath, llb.Readonly),
            llb.AddHostBindMount("/var/run/docker.sock", "/var/run/docker.sock"),
        )

        configStoreState = runState.GetMount(bctx.ConfigStorePath)
        dockerComposeState := runState.GetMount(bctx.DockerComposePath)
		dockerComposeStates = append(dockerComposeStates, dockerComposeState)

        return configStoreState
    }

	// create a map from service name to config store state
	serviceConfigStoreState := make(map[string]llb.State)
	

	// List all directories under the services directory

	// hookScript name should follow:
	//    "/services/${ACTION}/${SERVICE}.sh"
    serviceFiles, err := ioutil.ReadDir(bctx.ScriptsPath + "/services/" + "setup")
    if err != nil {
        log.Fatalf("Failed to read hook directory: %v", err)
    }
	for _, serviceFile := range serviceFiles {
		if serviceFile.IsDir() {
			continue
		}
		// check suffix
		
		if serviceFile.Name()[len(serviceFile.Name())-3:] != ".sh" {
			// log to stderr
			log.Printf("Skipping file %s, not a shell script", serviceFile.Name())
			continue
		}

		// don't forget to trim the .sh extension
		serviceName := serviceFile.Name()[:len(serviceFile.Name())-3]
		servicePath := "/services/setup/" + serviceName + ".sh"
		hostServicePath := bctx.ScriptsPath + servicePath

		// create configstore state for each service
		serviceConfigStoreState[serviceName] = configStore

		hashScript, err := hashFile(hostServicePath)
		if err != nil {
			return nil, fmt.Errorf("Failed to hash file: %v", err)
		}

		serviceConfigStoreState[serviceName] = runHook(serviceConfigStoreState[serviceName],
			llb.Shlex(fmt.Sprintf("bash /mnt%s %s", servicePath, hashScript)),
		)
	}

	// relationScript name should follow:
	//    "/relations/${RELATION_NAME}/${BASE}-${TARGET}/{base,target}.sh"

	var relationStates []llb.State

	relationDirs, err := ioutil.ReadDir(bctx.ScriptsPath + "/relations")
    if err != nil {
        log.Fatalf("Failed to read relations directory: %v", err)
    }


	for _, relationDir := range relationDirs {
		if ! relationDir.IsDir() {
			continue
		}
		relationName := relationDir.Name()
		relationPath := "/relations/" + relationName
		hostRelationPath := bctx.ScriptsPath + relationPath

		relationServicesDirs, err := ioutil.ReadDir(hostRelationPath)
		if err != nil {
			log.Fatalf("Failed to read relation directory %s: %v", hostRelationPath, err)
		}

		for _, relationServicesDir := range relationServicesDirs {
			if ! relationServicesDir.IsDir() {
				continue
			}

			relationServicesName := relationServicesDir.Name()
			relationServicesPath := relationPath + "/" + relationServicesName
			hostRelationServicesPath := hostRelationPath + "/" + relationServicesName

			services := strings.Split(relationServicesName, "-")
			var serviceName = make(map[string]string)
			serviceName["base"] = services[0]
			serviceName["target"] = services[1]
			
			
			relationData := llb.Scratch()
			relationDataPath := "/" + relationServicesName + "/" + relationName

			relationData = relationData.File(
				llb.Copy(
					relations,
					relationDataPath,
					"/",
					&llb.CopyInfo{CopyDirContentsOnly: true},
				),
			)

			relationInfo := RelationInfo{
				State: relationData,
				Name: relationName,
				Services: services,
			}

			for _, label := range []string{"target", "base"} {

				// check if ${label}.sh exists in the relationServicesPath
				relationScriptPath := relationServicesPath + "/" + label + ".sh"
				hostRelationScriptPath := hostRelationServicesPath + "/" + label + ".sh"

				if ! fileExists(hostRelationScriptPath) {
					continue
				}


				hashScript, err := hashFile(hostRelationScriptPath)
				if err != nil {
					return nil, fmt.Errorf("Failed to hash file: %v", err)
				}

				serviceConfigStoreState[serviceName[label]], relationData =
					runRelation(serviceConfigStoreState[serviceName[label]], relationInfo,
						llb.Shlex(fmt.Sprintf("bash /mnt%s %s", relationScriptPath, hashScript)),
					)
				relationInfo.State = relationData
			}

			relationOutputData := llb.Scratch().File(
				llb.Copy(
					relationData,
					"/",
					relationDataPath,
					&llb.CopyInfo{
						CopyDirContentsOnly: true,
						CreateDestPath: true,
					},
				),
			)
			relationStates = append(relationStates, relationOutputData)
		}
	}

	relationsMergedState := llb.Merge(relationStates)
	// merge all config stores from serviceConfigStoreState
	var configStoreStates []llb.State
    for _, v := range serviceConfigStoreState {
        configStoreStates = append(configStoreStates, v)
    }

	configStoreMergedState := llb.Merge(configStoreStates)
	dockerComposeMergedState := llb.Merge(dockerComposeStates)

	relationsOutput := llb.Scratch().File(
		llb.Copy(
			relationsMergedState,
			"/",
			"/relations/" + bctx.ProjectName,
			&llb.CopyInfo{
				CopyDirContentsOnly: true,
				CreateDestPath: true,
			},
		),
	)

	configStoreOutput := llb.Scratch().File(
		llb.Copy(
			configStoreMergedState,
			"/",
			"/configstore",
			&llb.CopyInfo{
				CopyDirContentsOnly: true,
				CreateDestPath: true,
			},
		),
	)
	dockerComposeOutput := llb.Scratch().File(
		llb.Copy(
			dockerComposeMergedState,
			"/",
			"/docker-compose",
			&llb.CopyInfo{
				CopyDirContentsOnly: true,
				CreateDestPath: true,
			},
		),
	)
    finalState := llb.Merge([]llb.State{configStoreOutput, relationsOutput, dockerComposeOutput})

    def, err := finalState.Marshal(ctx, llb.LinuxAmd64)
    if err != nil {
        return nil, fmt.Errorf("Failed to marshal LLB definition: %v", err)
    }

    return def, nil
}
