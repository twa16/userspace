package main

import (
	"time"
	"github.com/go-openapi/strfmt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/op/go-logging"
	"os"
	"github.com/spf13/viper"
	"github.com/fsouza/go-dockerclient"
)

//This is where I found the bug with Gogland haha (GO-3377)
//region Model Structs

type Space struct {
	ID            uint `gorm:"primary_key" json:"-"`           // Primary Key and ID of container
	PublicID      string `gorm:"index" json:"space_id"`        // Public UUID of this Space
	CreatedAt     time.Time `json:"-"`                         // Creation time
	ArchiveDate   time.Time `json:"archive_date,omitempty"`    // This is the timestamp of when the space was archived. This is set if the space was archived.
	Archived      bool `json:"archived"`                       // This value is true if the space was deleted as a result of inactivity. All data is lost but metadata is preserved.
	ImageID       string `json:"image_id"`                     // This is the image that is used by the container that contains the space. This is a link to SpaceImage.
	LastNetAccess string `json:"last_net_access,omitempty"`    // The time this space was last accessed over the network but not SSH. This may be empty if the space was never accessed.
	LastSSHAccess time.Time `json:"last_ssh_access,omitempty"` // The time this space was last accessed over SSH. This may be empty if the space was never accessed.
	OwnerID       *string `json:"owner_id"`                    // Unique ID of the user that owns the Space. This is a link to User.
	HostID        uint `json:"host_id"`                        // ID of the host that contains this space
	FriendlyName  string `json:"space_name"`                   // Friendly name of this space
	ContainerID   string `json:"space_id"`                     // ID of Docker container running this space
	SpaceState    string `json:"space_state"`                  // Running State of Space (running, paused, archived, error)
	SSHAddress    string `json:"ssh_address"`                  // Address that should be used to SSH into the Space.
	SSHPort       string `json:"ssh_port"`                     // Port that should be used to SSH into the Space.
	SSHKeyID      uint `json: "ssh_key_id"`                    // ID of the SSH Key that this container is using
	PortLinks     []SpacePortLink `json: "port_links"`	   // Shows what external ports are bound to the ports on the space
}

type SpacePortLink struct {
	ID              uint `gorm:"primary_key" json:"-"` // Primary Key and ID of container
	CreatedAt       time.Time `json:"-"`
	SpacePort       uint16 `json:"space_port"`
	ExternalPort    uint16 `json:"external_port;unique_index:idx_externaladdress"`
	ExternalAddress string `json: "external_address;unique_index:idx_externaladdress"`
	DisplayAddress  string `json: "external_display_address"`
}

//Authentication Token
type AuthenticationToken struct {
	ID             uint `gorm:"primary_key" json:"-"` //Primary Key
	CreatedAt      time.Time `json:"-"`               //Creation time
	ExpirationTime int64 `json:"expiration_time"`     // Unix time representation of when this token will be inactivated.
	Token          string `json:"token" gorm:"index"` // Token that is to be used in requests.
	UserID         string `json:"user_id"`            // ID of user this token represents
}

// SpaceImage
type SpaceImage struct {
	ID          uint `gorm:"primary_key" json:"-"`     //Primary Key
	CreatedAt   time.Time `json:"-"`                   //Creation time
	Active      bool `json:"active"`                   // If this is set to false, the user cannot use the image and is only kept to avoid breaking older spaces.
	Description *string `json:"description"`           // Friendly description of this image.
	DockerImage *string `json:"docker_image"`          // This is the full URI of the docker image.
	ImageID     *string `json:"image_id" gorm:"index"` // Unique ID of the image
	Name        *string `json:"name"`                  // Friendly name of this image.
}

// SpaceUsageReport This object stores the metrics for a space at a specific point in time. The reports are not reset each time therefore the difference between two reports will show the increase in the time between the reports.
type SpaceUsageReport struct {
	ID              uint `gorm:"primary_key" json:"-"` //Primary Key
	CreatedAt       time.Time `json:"-"`               //Creation time
	ContainerID     string `json:"container_id"`       // ID of the container
	DiskUsageBytes  int64 `json:"disk_usage_bytes"`    // Number of bytes that the space is taking up on disk.
	NetworkInBytes  int64 `json:"network_in_bytes"`    // Number of bytes that the space has received over the network. This does include SSH.
	NetworkOutBytes int64 `json:"network_out_bytes"`   // Number of bytes that the space has sent over the network. This includes SSH.
	ReportID        int64 `json:"report_id"`           // ID of the report
	SSHSessionCount int64 `json:"ssh_session_count"`   // This is the number of SSH sessions the space has received.
	Timestamp       time.Time `json:"timestamp"`       // Time this data was recorded
}

//UserPublicKey Represents a stored user public ssh key
type UserPublicKey struct {
	ID        uint `gorm:"primary_key" json:"-"`    // Primary Key
	PublicID  string `gorm:"index" json:"space_id"` // Public UUID of this Key
	CreatedAt time.Time `json:"-"`                  // Creation time
	OwnerID   string `json:"user_id"`               // ID of user tha owns this key
	Name      string `json:"name"`                  // Friendly name of this key
	PublicKey string `json:"public_key`             // Public key
}

// User User Object
type User struct {
	ID                        uint `gorm:"primary_key" json:"-"`                    //Primary Key
	CreatedAt                 time.Time `json:"-"`                                  //Creation Time
	UpdatedAt                 time.Time `json:"-"`                                  //Last Update time
	AuthenticationBackendLink string `json:"authentication_backend_link,omitempty"` // This is the field that links the user to the backend authentication service. In the initial system this stores the "netid" of the user that is used by CAS and LDAP.
	ExternallyAuthentication  bool `json:"externally_authentication"`               // If true, this user is authenticated against an external service which means there will be an authentication_backend_link but not a password.
	LastLoginTimestamp        strfmt.Date `json:"last_login_timestamp,omitempty"`   // The last time the user logged in. This is blank if the user has never logged in.
	Password                  string `json:"password,omitempty"`                    // BCrypt hash of the user password. This is only set if the user is not externally authenticated.
	UserID                    *string `json:"user_id"`                              // Unique ID of the user
}

//DockerInstance Struct representing a docker instance to use for containers
type DockerInstance struct {
	ID                     uint `gorm:"primary_key" json:"-"`       //Primary Key
	CreatedAt              time.Time `json:"-"`                     //Creation Time
	UpdatedAt              time.Time `json:"-"`                     //Last Update time
	Name                   string `json:"name"`                     //Friendly name of this docker instance
	ConnectionType         string `json:"connection_type"`          //Type of connection to use when connecting a docker instance (local,tls)
	Endpoint               string `json:"sock_path"`                //Path to the sock if the connection type is local or remote address if the type is tls
	CaCertPath             string `json:"ca_cert_path"`             //Path to the CA certificate if the connection type is tls
	ClientCertPath         string `json:"client_cert_path"`         //Path to the Client certificate if the connection type is tls
	ClientKeyPath          string `json:"client_key_path"`          //Path to the Client key if the connection type is tls
	IsConnected            bool   `json:"is_connected"`             //This is true if the daemon is reporting it is connected to the Docker host
	DockerClient           *docker.Client `gorm:"-" json:"-"`       //Connection to the Docker instance
	ExternalAddress        string `json:"external_address"`         //External address that the spaces will use
	ExternalDisplayAddress string `json:"external_display_address"` //External addresses that users will see
}

//endregion

//region Internal Structs

//endregion

//This should only be 4 chars or you have to change our fancy banner
var VERSION = "0.1A"
var log = logging.MustGetLogger("userspace-daemon")
var database *gorm.DB

func main() {
	Init()
}

//All code that would normally be in main() is put here in case we want to separate this into another package so it can be used as a library
func Init() {
	initLogging()
	log.Infof("\n"+"====================================\n"+"== Userspace Daemon               ==\n"+"== Version: %s                  ==\n"+"== Manuel Gauto(github.com/twa16) ==\n"+"====================================\n", VERSION)

	//Load the Configuration
	loadConfig()

	//Init DB
	log.Info("Connecting to database...")
	db, err := gorm.Open("sqlite3", "./dev.db")
	database = db
	defer database.Close()
	if err != nil {
		log.Fatalf("Failed to connect to database. Error: %s\n", err.Error())
		os.Exit(1)
	}

	//Migrate Models
	log.Info("Migrating Models...")
	database.AutoMigrate(&Space{})
	database.AutoMigrate(&SpacePortLink{})
	database.Model(&Space{}).Related(&SpacePortLink{})
	database.AutoMigrate(&AuthenticationToken{})
	database.AutoMigrate(&SpaceImage{})
	database.AutoMigrate(&SpaceUsageReport{})
	database.AutoMigrate(&User{})
	database.AutoMigrate(&DockerInstance{})
	database.AutoMigrate(&UserPublicKey{})
	log.Info("Migration Complete.")

	initDockerHosts(database)
	startAPI()
}

func initLogging() {

	// Example format string. Everything except the message has a custom color
	// which is dependent on the log level. Many fields have a custom output
	// formatting too, eg. the time returns the hour down to the milli second.
	var format = logging.MustStringFormatter(
		`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
	)
	backend := logging.NewLogBackend(os.Stderr, "", 0)

	// For messages written to backend2 we want to add some additional
	// information to the output, including the used log level and the name of
	// the function.
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)
}

//loadConfig I bet you can guess what this function does
func loadConfig() {
	viper.SetConfigName("config")                // name of config file (without extension)
	viper.AddConfigPath("./config")              // path to look for the config file in
	viper.AddConfigPath("/etc/userspace/config") // path to look for the config file in
	viper.AddConfigPath(".")                     // optionally look for config in the working directory

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {
		log.Fatalf("Fatal error config file: %s \n", err) // Handle errors reading the config file
		panic(err)
	}

	log.Infof("Using config file: %s", viper.ConfigFileUsed())
	for _, key := range viper.AllKeys() {
		log.Infof("Loaded: %s as %s", key, viper.GetString(key))
	}
	//viper.SetDefault("k", "v")
}
