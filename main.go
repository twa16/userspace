package main

import (
	"time"
	"github.com/go-openapi/strfmt"
)

//Space
type Space struct {
	//Primary Key
	ID 	uint `gorm:"primary_key" json:"-"`

	// This is the timestamp of when the space was archived. This is set if the space was archived.
	ArchiveDate time.Time `json:"archive_date,omitempty"`

	// This value is true if the space was deleted as a result of inactivity. All data is lost but metadata is preserved.
	// Required: true
	Archived bool `json:"archived"`

	// Timestamp representing the instance in time that the space was created.
	// Required: true
	CreatedAt time.Time `json:"creation_date"`

	// This is the image that is used by the container that contains the space. This is a link to SpaceImage.
	// Required: true
	ImageID string `json:"image_id"`

	// The time this space was last accessed over the network but not SSH. This may be empty if the space was never accessed.
	LastNetAccess string `json:"last_net_access,omitempty"`

	// The time this space was last accessed over SSH. This may be empty if the space was never accessed.
	LastSSHAccess time.Time `json:"last_ssh_access,omitempty"`

	// Unique ID of the user that owns the Space. This is a link to User.
	// Required: true
	OwnerID *string `json:"owner_id"`

	// Unique ID of the Space
	// Required: true
	SpaceID string `json:"space_id" gorm:"index"`

	// Address that should be used to SSH into the Space.
	// Required: true
	SSHAddress string `json:"ssh_address"`

	// Port that should be used to SSH into the Space.
	// Required: true
	SSHPort string `json:"ssh_port"`
}

//Authentication Token
type AuthenticationToken struct {
	//Primary Key
	ID 	uint `gorm:"primary_key" json:"-"`

	//Creation time
	CreatedAt time.Time `json:"-"`

	// Unix time representation of when this token will be inactivated.
	// Required: true
	ExpirationTime int64 `json:"expiration_time"`

	// Token that is to be used in requests.
	// Required: true
	Token string `json:"token" gorm:"index"`

	// ID of user this token represents
	// Required: true
	UserID string `json:"user_id"`
}

// SpaceImage
type SpaceImage struct {
	//Primary Key
	ID 	uint `gorm:"primary_key" json:"-"`

	//Creation time
	CreatedAt time.Time `json:"-"`

	// If this is set to false, the user cannot use the image and is only kept to avoid breaking older spaces.
	// Required: true
	Active bool `json:"active"`

	// Friendly description of this image.
	// Required: true
	Description *string `json:"description"`

	// This is the full URI of the docker image.
	// Required: true
	DockerImage *string `json:"docker_image"`

	// Unique ID of the image
	// Required: true
	ImageID *string `json:"image_id" gorm:"index"`

	// Friendly name of this image.
	// Required: true
	Name *string `json:"name"`
}

// SpaceUsageReport This object stores the metrics for a space at a specific point in time. The reports are not reset each time therefore the difference between two reports will show the increase in the time between the reports.
type SpaceUsageReport struct {
	//Primary Key
	ID 	uint `gorm:"primary_key" json:"-"`

	//Creation time
	CreatedAt time.Time `json:"-"`

	// ID of the container
	// Required: true
	ContainerID string `json:"container_id"`

	// Number of bytes that the space is taking up on disk.
	// Required: true
	DiskUsageBytes int64 `json:"disk_usage_bytes"`

	// Number of bytes that the space has received over the network. This does include SSH.
	// Required: true
	NetworkInBytes int64 `json:"network_in_bytes"`

	// Number of bytes that the space has sent over the network. This includes SSH.
	// Required: true
	NetworkOutBytes int64 `json:"network_out_bytes"`

	// ID of the report
	// Required: true
	ReportID int64 `json:"report_id"`

	// This is the number of SSH sessions the space has received.
	// Required: true
	SSHSessionCount int64 `json:"ssh_session_count"`

	// Time this data was recorded
	// Required: true
	Timestamp time.Time `json:"timestamp"`
}

// User User Object
type User struct {
	//Primary Key
	ID 	uint `gorm:"primary_key" json:"-"`

	//Creation time
	CreatedAt time.Time `json:"-"`

	//Last Update time
	UpdatedAt time.Time `json:"-"`

	// This is the field that links the user to the backend authentication service. In the initial system this stores the "netid" of the user that is used by CAS and LDAP.
	AuthenticationBackendLink string `json:"authentication_backend_link,omitempty"`

	// If true, this user is authenticated against an external service which means there will be an authentication_backend_link but not a password.
	// Required: true
	ExternallyAuthentication bool `json:"externally_authentication"`

	// The last time the user logged in. This is blank if the user has never logged in.
	LastLoginTimestamp strfmt.Date `json:"last_login_timestamp,omitempty"`

	// BCrypt hash of the user password. This is only set if the user is not externally authenticated.
	Password string `json:"password,omitempty"`

	// Unique ID of the user
	// Required: true
	UserID *string `json:"user_id"`
}


