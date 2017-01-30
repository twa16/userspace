package main

import (
	"github.com/fsouza/go-dockerclient"
	"github.com/jinzhu/gorm"
	"sync"
)

//Contains running instances of docker hosts
var DockerInstances []*DockerInstance

func initDockerHosts(db *gorm.DB) {
	instances := getAllDockerInstanceConfigurations(db)
	log.Infof("Initiating Connections to %d Docker Host(s)\n", len(instances))
	for _, instance := range instances {
		_, err := addAndConnectToDockerInstance(db, &instance)
		if err != nil {
			log.Critical("Adding Docker Host %s Failed: %s\n", instance.Name, err.Error())
			continue
		}
		log.Infof("Connected to Docker Host: %s\n", instance.Name)
	}
	log.Info("Initialized All Docker Hosts!")
}

var dockerInstanceSliceLock sync.Mutex
func addAndConnectToDockerInstance(db *gorm.DB, instance *DockerInstance) (*DockerInstance, error) {
	//It is technically possible for two hosts to be added at once, so let's lock the slice
	dockerInstanceSliceLock.Lock()
	defer dockerInstanceSliceLock.Unlock()
	_, err := startDockerClient(instance)
	if err != nil {
		log.Fatalf("Error Starting Docker Client: %s\n", err.Error())
		return nil, err
	}
	db.Save(&instance)
	DockerInstances = append(DockerInstances, instance)
	return instance, nil
}

func getAllDockerInstanceConfigurations(db *gorm.DB) []DockerInstance {
	configs := []DockerInstance{}
	db.Find(&configs)
	return configs
}

func startDockerClient(instance *DockerInstance) (*docker.Client, error) {
	log.Infof("Connecting to Docker Host %s using connection type %s\n", instance.Name, instance.ConnectionType)
	var cli *docker.Client
	var err error
	//Check for type and init accordingly
	if instance.ConnectionType == "local" {
		if instance.Endpoint == "" {
			cli, err = docker.NewClientFromEnv()
		} else {
			cli, err = docker.NewClient(instance.Endpoint)
		}
	} else {
		cli, err = docker.NewTLSClient(instance.Endpoint, instance.ClientCertPath, instance.ClientKeyPath, instance.CaCertPath)
	}

	if err != nil {
		log.Criticalf("Failed to start docker client for %s: %s\n", instance.Name, err.Error())
		return nil, err
	}
	//Put new connection data into the struct
	instance.IsConnected = true
	instance.DockerClient = cli

	env, _ := cli.Version()
	log.Info("Connection Suceeded! API Version: "+env.Get("ApiVersion"))
	return cli, err
}