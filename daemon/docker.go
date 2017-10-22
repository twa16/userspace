/*
 * Copyright 2017 Manuel Gauto (github.com/twa16)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package userspaced

import (
	"context"
	"math/rand"
	"strconv"
	"sync"

	"github.com/fsouza/go-dockerclient"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

//Contains running instances of docker hosts
var DockerInstances []*DockerInstance

//initDockerHosts Pulls docker hosts from the DB, adds them to the cache and connects to them
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

//addAndConnectToDockerInstance Adds a new host to in-memory cache and establishes connection to it.
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
func getHostByID(hostID uint) *DockerInstance {
	for i, instance := range DockerInstances {
		if instance.ID == hostID {
			return DockerInstances[i]
		}
	}
	return nil
}

//getImageByID Helper method that gets an image by id
func getImageByID(db *gorm.DB, imageID uint) SpaceImage {
	var image SpaceImage
	db.Find(&image, imageID)
	return image
}

//checkImageExists Check if an image exists
func checkImageExists(db *gorm.DB, imageID uint) bool {
	var image SpaceImage
	return !db.Find(&image, imageID).RecordNotFound()
}

//securePortForSpace Picks an open port between 20000 and 30000. Saves new PortLink
func securePortForSpace(db *gorm.DB, space *Space, destPort uint16) *SpacePortLink {
	log.Debugf("Attempting to secure port for %d: %d\n", space.ID, destPort)
	spaceHost := getHostByID(space.HostID)
	originalPort := space.PortLinks
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
		log.Debugf("Trying to secure port %d for %d\n", portTry, space.ID)
		space.PortLinks = append(originalPort, portMapping)
		//db.Model(&space).Related(&space.PortLinks)
		//Try to update and see if we get an error
		//Maybe we should add an attempt cap here
		//err := db.Save(&space).Error
		if isPortOpen(db, portMapping.ExternalAddress, portMapping.ExternalPort) {
			err := db.Save(&space).Error
			if err != nil {
				log.Criticalf("Error Saving Port Mapping: %s\n", err.Error())
			} else {
				log.Infof("Secured port mapping for space %d: %d -> %d\n", space.ID, portTry, destPort)
				return &portMapping
			}
		} else {
			log.Infof("Port %d is taken\n", portTry)
		}
	}
	//This should never happen unless something went horribly wrong
	return nil
}

func isPortOpen(db *gorm.DB, externalAddress string, port uint16) bool {
	var holder SpacePortLink
	return db.Where(&SpacePortLink{ExternalAddress: externalAddress, ExternalPort: port}).First(&holder).RecordNotFound()
}

//TODO: Make this do the thing. Returns first instance since when this was written spaces weren't created yet. This will probably be done with raw sql.
//selectLeastOccupiedHost Returns the host that has the fewest number of instances.
func selectLeastOccupiedHost(db *gorm.DB) (*DockerInstance, error) {
	if len(DockerInstances) == 0 {
		return nil, errors.New("No Hosts Have Been Added!")
	}
	return DockerInstances[0], nil
}

//startSpace Creates and starts a new space
func startSpace(db *gorm.DB, space *Space, creationStatusChan chan string) (error, *Space) {
	//======Initialization Steps=====
	//Check if the requested image exists
	if !checkImageExists(db, space.ImageID) {
		creationStatusChan <- "Error: Invalid Image"
		return errors.New("Invalid Image Specified"), nil
	}
	//Pick a host
	dockerHost, err := selectLeastOccupiedHost(db)
	if err != nil {
		log.Critical("No hosts have been added.")
		return err, nil
	}
	space.HostID = dockerHost.ID
	space.SpaceState = "creation started"
	client := dockerHost.DockerClient
	//Save it
	db.Create(&space)
	creationStatusChan <- "Host Chosen"
	log.Infof("Selected Host %d for space %d\n", space.HostID, space.ID)

	//======Container Config=====
	var containerConfig docker.Config
	//Set the image
	containerConfig.Image = getImageByID(db, space.ImageID).DockerImage

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
	sshPortLink := securePortForSpace(db, space, 22)
	servicePortLink := securePortForSpace(db, space, 1337)
	//Setup Port Maps
	//Forward a dynamic host port to container. Listen on localhost so that nginx can proxy.
	hostConfig.PortBindings = make(map[docker.Port][]docker.PortBinding)
	hostConfig.PortBindings["22/tcp"] = append(hostConfig.PortBindings["22/tcp"], docker.PortBinding{HostIP: "127.0.0.1", HostPort: strconv.Itoa(int(sshPortLink.ExternalPort))})
	hostConfig.PortBindings["1337/tcp"] = append(hostConfig.PortBindings["1337/tcp"], docker.PortBinding{HostIP: "127.0.0.1", HostPort: strconv.Itoa(int(servicePortLink.ExternalPort))})
	hostConfig.PortBindings["1337/udp"] = append(hostConfig.PortBindings["1337/udp"], docker.PortBinding{HostIP: "127.0.0.1", HostPort: strconv.Itoa(int(servicePortLink.ExternalPort))})
	//Save PortLinks
	sshPortLink.ExternalAddress = dockerHost.ExternalAddress
	sshPortLink.DisplayAddress = dockerHost.ExternalDisplayAddress
	sshPortLink.SpacePort = 22

	servicePortLink.ExternalAddress = dockerHost.ExternalAddress
	servicePortLink.DisplayAddress = dockerHost.ExternalDisplayAddress
	servicePortLink.SpacePort = 1337

	space.PortLinks = append(space.PortLinks, *sshPortLink)
	space.PortLinks = append(space.PortLinks, *servicePortLink)
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
	if err != nil {
		log.Criticalf("Failed to create container for space %d\n", space.ID)
		log.Debug(err)
		space.SpaceState = "Error Creating"
		db.Save(&space)
		creationStatusChan <- "Error: Error Creating Container"
		return err, nil
	}
	creationStatusChan <- "Container Created"
	//Set container
	space.ContainerID = c.ID
	//At first, let us keep all containers alive
	//TODO: Make this configurable and with a quota
	space.KeepAlive = true
	space.SpaceState = "created"
	db.Save(&space)
	log.Infof("Created container for space %d: %s\n", space.ID, space.ContainerID)

	err = client.StartContainer(space.ContainerID, nil)
	if err != nil {
		log.Criticalf("Error starting container for space %d: %s\n", space.ID, err.Error())
		space.SpaceState = "error starting"
		db.Save(&space)
	} else {
		log.Infof("Container for Space %d started: %s\n", space.ID, space.ContainerID)
		space.SpaceState = "running"
		db.Save(&space)
	}
	creationStatusChan <- "Creation Complete"
	//execInSpace(database, *space, []string{"touch", "/root/testblop"})
	return nil, space
}

//execInSpace Executes a command in a space
func execInSpace(db *gorm.DB, space Space, command []string) error {
	dockerHost := getHostByID(space.HostID)

	execOptions := docker.CreateExecOptions{}
	execOptions.Cmd = command
	execOptions.AttachStderr = false
	execOptions.AttachStdin = false
	execOptions.AttachStdout = false
	execOptions.Tty = false
	exec, err := dockerHost.DockerClient.CreateExec(execOptions)
	if err != nil {
		log.Warningf("Error Executing Command on Host %s: %s\n", dockerHost.Name, err.Error())
		return err
	}
	err = dockerHost.DockerClient.StartExec(exec.ID, docker.StartExecOptions{})
	if err != nil {
		log.Warningf("Error Executing Command on Host %s: %s\n", dockerHost.Name, err.Error())
		return err
	}
	return nil
}

//startDockerClient Opens a connection to a docker instance
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
	log.Info("Connection Suceeded! API Version: " + env.Get("ApiVersion"))
	return cli, err
}

//ensureStarterImages Pulls docker images needed for spaces to all hosts
func ensureStarterImages(db *gorm.DB) {
	ubuntuImage := SpaceImage{}
	db.Where("docker_image = ? AND docker_image_tag = ?", "userspace/ubuntu", "latest").First(&ubuntuImage)
	if ubuntuImage.Active == false {
		log.Info("Downloading Start Images")
		ubuntuImage.Active = true
		ubuntuImage.Description = "Basic Ubuntu Image"
		ubuntuImage.DockerImage = "userspace/ubuntu"
		ubuntuImage.DockerImageTag = "latest"
		ubuntuImage.Name = "Ubuntu"
		db.Create(&ubuntuImage)
	}
}

//downloadDockerImages Download an image to host
func downloadDockerImages(db *gorm.DB) {
	images := []SpaceImage{}
	db.Find(&images)
	for _, image := range images {
		for _, instance := range DockerInstances {
			if instance.IsConnected {
				pullDockerImage(instance.DockerClient, image.DockerImage, image.DockerImageTag)
				log.Infof("Downloaded image to %s\n", instance.Name)
			} else {
				log.Warningf("Skipping %s as it is not connected!\n", instance.Name)
			}
		}
	}
}

//pullDockerImage Pulls a docker image from the hub
func pullDockerImage(dClient *docker.Client, image string, tag string) error {
	pullOptions := docker.PullImageOptions{Repository: image, Tag: tag}
	authOptions := docker.AuthConfiguration{}
	err := dClient.PullImage(pullOptions, authOptions)
	return err
}

//RemoveSpace Removes a Space's container and DB entry
func RemoveSpace(db *gorm.DB, space Space) error {
	//Get Host Docker Connection
	hostID := space.HostID
	dockerHost := getHostByID(hostID)
	//Ensure the host is connected
	if !dockerHost.IsConnected {
		log.Critical("Attempted to remove container %s from disconnected host.")
		return errors.New("Attempted to remove contaienr from disconnected host.")
	}
	//Set the space state
	space.SpaceState = "deleting"
	db.Save(&space)

	//Only remove the container if the container is not already dead
	if space.SpaceState != "error" {
		//Stop the container
		dClient := dockerHost.DockerClient
		err := dClient.StopContainer(space.ContainerID, 30)
		//Catch any errors
		if err != nil {
			log.Criticalf("Error stopping container %s: %s", space.ContainerID, err.Error())
			//return err
		} else {
			//Remove the container
			removeOptions := docker.RemoveContainerOptions{
				ID:            space.ContainerID,
				RemoveVolumes: true,
				Force:         true,
				Context:       context.Background(),
			}
			err = dClient.RemoveContainer(removeOptions)
			if err != nil {
				log.Criticalf("Error removing container %s: %s", space.ContainerID, err.Error())
				//return err
			}
		}
	}
	//Remove the db object
	err := db.Delete(&space).Error
	if err != nil {
		log.Criticalf("Error removing space record for %d\n", space.ID, err.Error())
		return err
	}
	return nil
}
