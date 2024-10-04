package main

import (
	"fmt"
	"os"
	"github.com/0k/0k-compose-builder/internal"
)


func NewBuildContext() (*internal.BuildContext, error) {
    runnerImage := getEnv("COMPOSE_DOCKER_IMAGE", "docker.0k.io/compose:latest")

    projectName := os.Getenv("PROJECT_NAME")
    if projectName == "" {
        return nil, fmt.Errorf("PROJECT_NAME environment variable is required")
    }

    charmStorePath := getEnv("CHARM_STORE", "/srv/charm-store")
    if _, err := os.Stat(charmStorePath); os.IsNotExist(err) {
        return nil, fmt.Errorf("CHARM_STORE path %s does not exist", charmStorePath)
    }

    configStorePath := getEnv("CONFIGSTORE", "/srv/datastore/config")
    if _, err := os.Stat(configStorePath); os.IsNotExist(err) {
        return nil, fmt.Errorf("CONFIGSTORE path %s does not exist", configStorePath)
    }

    relationDataPath := getEnv("RELATION_DATA", "/var/lib/compose/relations")
    dockerComposePath := getEnv("DOCKER_COMPOSE_FRAGMENTS", "/var/lib/compose/docker-compose-fragments")

    return &internal.BuildContext{
        RunnerImage:       runnerImage,
        ProjectName:       projectName,
        CharmStorePath:    charmStorePath,
        ConfigStorePath:   configStorePath,
        RelationDataPath:  relationDataPath,
        DockerComposePath: dockerComposePath,
    }, nil
}


// getEnv retrieves the environment variable or returns the default value
func getEnv(key, defaultValue string) string {
    if value, exists := os.LookupEnv(key); exists && value != "" {
        return value
    }
    return defaultValue
}

