package main

import (
	"github.com/fsouza/go-dockerclient"
	"github.com/jinzhu/gorm"
	"sync"
	"github.com/docker/docker/api/types"
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

//getAllDockerInstanceConfigurations Gets all hosts from the db
func getAllDockerInstanceConfigurations(db *gorm.DB) []DockerInstance {
	configs := []DockerInstance{}
	db.Find(&configs)
	return configs
}

//getHostByID Helper method that gets a host from the db by id
func getHostByID(db *gorm.DB, hostID uint) *DockerInstance {
	var dockerHost *DockerInstance
	db.Find(dockerHost, hostID)
	return dockerHost
}

func getImageByID(db *gorm.DB, imageID string) SpaceImage {
	var image SpaceImage
	db.Find(&image, imageID)
	return image
}

func startSpace(db *gorm.DB, space Space) {
	//======Container Config=====
	var containerConfig docker.Config
	//Set the image
	containerConfig.Image = *getImageByID(db, space.ImageID).DockerImage

	//Empty placeholder struct
	var v struct{}

	//Create empty volume set
	//containerConfig.Volumes = make(map[string]struct{})
	//Ports
	//port80, _ := nat.NewPort("tcp", "80")
	containerConfig.ExposedPorts = make(map[docker.Port]struct{})
	containerConfig.ExposedPorts["22/tcp"] = v
	containerConfig.ExposedPorts["1337/tcp"] = v

	//=====Host Config======
	var hostConfig docker.HostConfig
	//Setup Port Maps
	//Forward a dynamic host port to container. Listen on localhost so that nginx can proxy.
	hostConfig.PortBindings = make(map[docker.Port][]docker.PortBinding)
	hostConfig.PortBindings["22/tcp"] = append(hostConfig.PortBindings["22/tcp"], docker.PortBinding{HostIP: "127.0.0.1", HostPort: externalPort})

	//======Network Config=====
	var networkConfig docker.NetworkingConfig

	//======Container Creation=====
	//Wrapup config
	var config docker.CreateContainerOptions
	config.Config = &containerConfig
	config.HostConfig = &hostConfig
	config.NetworkingConfig = &networkConfig
	config.Context = context.Background()
	//Create Container
	c, err := client.CreateContainer(config)
	return c, err
}



func execInSpace(db *gorm.DB, space Space, command []string) (error){
	dockerHost := getHostByID(db, space.HostID)

	execOptions := docker.CreateExecOptions{}
	execOptions.Cmd = command
	execOptions.AttachStderr = false
	execOptions.AttachStdin = false
	execOptions.AttachStdout = false
	execOptions.Tty = false
	_, err := dockerHost.DockerClient.CreateExec(execOptions)
	if err != nil {
		log.Warningf("Error Executing Command on Host %s: %s\n", dockerHost.Name, err.Error())
		return err
	}



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