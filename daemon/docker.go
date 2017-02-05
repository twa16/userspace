package main

import (
	"github.com/fsouza/go-dockerclient"
	"github.com/jinzhu/gorm"
	"sync"
	"context"
	"math/rand"
	"github.com/pkg/errors"
	"strconv"
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

//getImageByID Helper method that gets an image by id
func getImageByID(db *gorm.DB, imageID string) SpaceImage {
	var image SpaceImage
	db.Find(&image, imageID)
	return image
}

//checkImageExists Check if an image exists
func checkImageExists(db *gorm.DB, imageID string) bool {
	var image SpaceImage
	return !db.Find(&image, imageID).RecordNotFound()
}

//securePortForSpace Picks an open port between 20000 and 30000. Saves new PortLink
func securePortForSpace(db *gorm.DB, space *Space, destPort uint16) int {
	log.Debugf("Attempting to secure port for %u:%u\n", space.ID, destPort)
	spaceHost := getHostByID(db, space.HostID)
	for true {
		//Copy over the basics
		portMapping := SpacePortLink{}
		portMapping.ExternalAddress = spaceHost.ExternalAddress
		portMapping.DisplayAddress = spaceHost.ExternalDisplayAddress
		portMapping.SpacePort = destPort
		//Generate a new port
		var portTry = 20000 + rand.Intn(10000)
		//Set it and append the mapping
		portMapping.ExternalPort = uint16(portTry)
		log.Debugf("Trying to secure port %u for %u\n", portTry, space.ID)
		space.PortLinks = append(space.PortLinks, portMapping)
		//Try to update and see if we get an error
		//Maybe we should add an attempt cap here
		err := db.Update(&space).Error
		if err == nil {
			log.Info("Secured port mapping for space %u: %u -> %u\n", space.ID, portTry, destPort)
			return portTry
		} else {
			log.Info("Port %u is taken\n", portTry)
		}
	}

}

//TODO: Make this do the thing. Returns first instance since when this was written spaces weren't created yet. This will probably be done with raw sql.
//selectLeastOccupiedHost Returns the host that has the fewest number of instances.
func selectLeastOccupiedHost(db *gorm.DB) *DockerInstance {
	return DockerInstances[0]
}

func startSpace(db *gorm.DB, client *docker.Client, space Space) (error, *Space){
	//======Initialization Steps=====
	//Check if the requested image exists
	if !checkImageExists(db, space.ImageID) {
		return errors.New("Invalid Image Specified"), nil
	}
	//Pick a host
	space.HostID = selectLeastOccupiedHost(db).ID
	//Save it
	db.Create(&space)
	log.Infof("Select Host %u for space %u\n", space.HostID, space.ID)

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
	containerConfig.ExposedPorts["1337/udp"] = v

	//=====Host Config======
	var hostConfig docker.HostConfig
	//Secure Ports in DB
	sshExternalPort := securePortForSpace(db, &space, 22)
	serviceExternalPort := securePortForSpace(db, &space, 1337)
	//Setup Port Maps
	//Forward a dynamic host port to container. Listen on localhost so that nginx can proxy.
	hostConfig.PortBindings = make(map[docker.Port][]docker.PortBinding)
	hostConfig.PortBindings["22/tcp"] = append(hostConfig.PortBindings["22/tcp"], docker.PortBinding{HostIP: "127.0.0.1", HostPort: strconv.Itoa(sshExternalPort)})
	hostConfig.PortBindings["1337/tcp"] = append(hostConfig.PortBindings["1337/tcp"], docker.PortBinding{HostIP: "127.0.0.1", HostPort: strconv.Itoa(serviceExternalPort)})
	hostConfig.PortBindings["1337/udp"] = append(hostConfig.PortBindings["1337/udp"], docker.PortBinding{HostIP: "127.0.0.1", HostPort: strconv.Itoa(serviceExternalPort)})

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
	if err == nil {
		log.Fatalf("Failed to create container for space %u\n", space.ID)
		space.SpaceState = "Error Creating"
		db.Save(space)
		return err, nil
	}
	//Set container
	space.ContainerID = c.ID
	space.SpaceState = "Created"
	db.Save(&space)
	log.Infof("Created container for space %u: %s\n", space.ID, space.ContainerID)

	err = client.StartContainer(space.ContainerID, nil)
	if err != nil {
		log.Fatalf("Error starting container for space %u: %s\n", space.ID, space.ContainerID)
		space.SpaceState = "Error Starting"
		db.Save(space)
	} else {
		log.Infof("Container for Space %u started: %s\n", space.ID, space.ContainerID)
		space.SpaceState = "Running"
		db.Save(space)
	}
	return nil, &space
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