package instances

import (
	"github.com/gophercloud/gophercloud"
	db "github.com/gophercloud/gophercloud/openstack/db/v1/databases"
	"github.com/gophercloud/gophercloud/openstack/db/v1/users"
	"github.com/gophercloud/gophercloud/pagination"
)

// CreateOptsBuilder is the top-level interface for create options.
type CreateOptsBuilder interface {
	ToInstanceCreateMap() (map[string]interface{}, error)
}

// DatastoreOpts represents the configuration for how an instance stores data.
type DatastoreOpts struct {
	Version string `json:"version"`
	Type    string `json:"type"`
}

// ToMap converts a DatastoreOpts to a map[string]string (for a request body)
func (opts DatastoreOpts) ToMap() (map[string]interface{}, error) {
	return gophercloud.BuildRequestBody(opts, "")
}

// NetworkOpts is used within CreateOpts to control a new server's network attachments.
type NetworkOpts struct {
	// UUID of a nova-network to attach to the newly provisioned server.
	// Required unless Port is provided.
	UUID string `json:"net-id,omitempty"`

	// Port of a neutron network to attach to the newly provisioned server.
	// Required unless UUID is provided.
	Port string `json:"port-id,omitempty"`

	// V4FixedIP [optional] specifies a fixed IPv4 address to be used on this network.
	V4FixedIP string `json:"v4-fixed-ip,omitempty"`

	// V6FixedIP [optional] specifies a fixed IPv6 address to be used on this network.
	V6FixedIP string `json:"v6-fixed-ip,omitempty"`
}

// ToMap converts a NetworkOpts to a map[string]string (for a request body)
func (opts NetworkOpts) ToMap() (map[string]interface{}, error) {
	return gophercloud.BuildRequestBody(opts, "")
}

// AccessOpts structure for access parameters in order to enable public access and allowed cidrs
type AccessOpts struct {
	IsPublic     bool     `json:"is_public"`
	AllowedCidrs []string `json:"allowed_cirdrs"`
}

// ToMap converts an AccessOpt to a map[string]string (for a request body)
func (opts AccessOpts) ToMap() (map[string]interface{}, error) {
	return gophercloud.BuildRequestBody(opts, "")
}

// RestoreOpts for instance creation from exisiting backup
type RestoreOpts struct {
	BackupRef string `json:"backup_ref"`
}

// ToMap converts an RestoreOpts to a map[string]string (for a request body)
func (opts RestoreOpts) ToMap() (map[string]interface{}, error) {
	return gophercloud.BuildRequestBody(opts, "")
}

// CreateOpts is the struct responsible for configuring a new database instance.
type CreateOpts struct {
	// Either the integer UUID (in string form) of the flavor, or its URI
	// reference as specified in the response from the List() call. Required.
	FlavorRef string `json:"flavor_ref" required:"true"`
	// Specifies the volume size in gigabytes (GB). The value must be between 1
	// and 300. Required.
	Size int `json:"size" required:"true"`
	// Name of the instance to create. The length of the name is limited to
	// 255 characters and any characters are permitted. Optional.
	Name string `json:"name" required:"true"`
	// A slice of database information options.
	Databases db.CreateOptsBuilder `json:"database"`
	// A slice of user information options.
	Users users.CreateOptsBuilder `json:"user"`
	// Options to configure the type of datastore the instance will use. This is
	// optional, and if excluded will default to MySQL.
	Datastore *DatastoreOpts `json:"datastore" required:"true"`
	// Networks dictates how this server will be attached to available networks.
	Networks []NetworkOpts `json:"networks"`
	// Access Define how the database is exposed
	Access *AccessOpts `json:"access"`
	// Create an instance from a backup
	RestorePoint *RestoreOpts `json:"restore_point"`
	// ReplicaOf is the Id or name of an existing instance to replicate from
	ReplicaOf string `json:"replica_of"`
	// ReplicaCount is a the number of replica to create
	// if not provided and replicaOf set it will be 1 by default
	ReplicaCount int `json:"replica_count"`
}

// ToInstanceCreateMap will render a JSON map.
func (opts CreateOpts) ToInstanceCreateMap() (map[string]interface{}, error) {
	if opts.Size > 300 || opts.Size < 1 {
		err := gophercloud.ErrInvalidInput{}
		err.Argument = "instances.CreateOpts.Size"
		err.Value = opts.Size
		err.Info = "Size (GB) must be between 1-300"
		return nil, err
	}

	if opts.FlavorRef == "" {
		return nil, gophercloud.ErrMissingInput{Argument: "instances.CreateOpts.FlavorRef"}
	}

	instance := map[string]interface{}{
		"volume":    map[string]int{"size": opts.Size},
		"flavorRef": opts.FlavorRef,
	}

	if opts.Name != "" {
		instance["name"] = opts.Name
	}
	if opts.Databases != nil {
		dbs, err := opts.Databases.ToDBCreateMap()
		if err != nil {
			return nil, err
		}
		instance["databases"] = dbs["databases"]
	}
	if opts.Users != nil {
		users, err := opts.Users.ToUserCreateMap()
		if err != nil {
			return nil, err
		}
		instance["users"] = users["users"]
	}
	if opts.Datastore != nil {
		datastore, err := opts.Datastore.ToMap()
		if err != nil {
			return nil, err
		}
		instance["datastore"] = datastore
	}

	if len(opts.Networks) > 0 {
		networks := make([]map[string]interface{}, len(opts.Networks))
		for i, net := range opts.Networks {
			var err error
			networks[i], err = net.ToMap()
			if err != nil {
				return nil, err
			}
		}
		instance["nics"] = networks
	}

	// Add access parameter (enable public instance or not, set list of available cidrs)
	if opts.Access != nil {
		access, err := opts.Access.ToMap()
		if err != nil {
			return nil, err
		}
		instance["access"] = access
	}

	// If the instance to create is a restoration of another one
	if opts.RestorePoint != nil {
		if opts.RestorePoint.BackupRef == "" {
			return nil, gophercloud.ErrMissingInput{Argument: "restore_point.backup_ref"}
		}
		instance["RestorePoint"] = map[string]interface{}{
			"backup_ref": opts.RestorePoint.BackupRef,
		}
	}

	// The database instance from which the instance must replicate the data
	if opts.ReplicaOf != "" {
		instance["replica_of"] = opts.ReplicaOf
	}

	// Default to 1 if not provided to the API so check for value over 1 otherwise no need to set the field
	if opts.ReplicaCount > 1 {
		instance["replica_count"] = opts.ReplicaCount
	}

	return map[string]interface{}{"instance": instance}, nil
}

// Create asynchronously provisions a new database instance. It requires the
// user to specify a flavor and a volume size. The API service then provisions
// the instance with the requested flavor and sets up a volume of the specified
// size, which is the storage for the database instance.
//
// Although this call only allows the creation of 1 instance per request, you
// can create an instance with multiple databases and users. The default
// binding for a MySQL instance is port 3306.
func Create(client *gophercloud.ServiceClient, opts CreateOptsBuilder) (r CreateResult) {
	b, err := opts.ToInstanceCreateMap()
	if err != nil {
		r.Err = err
		return
	}
	resp, err := client.Post(baseURL(client), &b, &r.Body, &gophercloud.RequestOpts{OkCodes: []int{200}})
	_, r.Header, r.Err = gophercloud.ParseResponse(resp, err)
	return
}

// List retrieves the status and information for all database instances.
func List(client *gophercloud.ServiceClient) pagination.Pager {
	return pagination.NewPager(client, baseURL(client), func(r pagination.PageResult) pagination.Page {
		return InstancePage{pagination.LinkedPageBase{PageResult: r}}
	})
}

// Get retrieves the status and information for a specified database instance.
func Get(client *gophercloud.ServiceClient, id string) (r GetResult) {
	resp, err := client.Get(resourceURL(client, id), &r.Body, nil)
	_, r.Header, r.Err = gophercloud.ParseResponse(resp, err)
	return
}

// Delete permanently destroys the database instance.
func Delete(client *gophercloud.ServiceClient, id string) (r DeleteResult) {
	resp, err := client.Delete(resourceURL(client, id), nil)
	_, r.Header, r.Err = gophercloud.ParseResponse(resp, err)
	return
}

// EnableRootUser enables the login from any host for the root user and
// provides the user with a generated root password.
func EnableRootUser(client *gophercloud.ServiceClient, id string) (r EnableRootUserResult) {
	resp, err := client.Post(userRootURL(client, id), nil, &r.Body, &gophercloud.RequestOpts{OkCodes: []int{200}})
	_, r.Header, r.Err = gophercloud.ParseResponse(resp, err)
	return
}

// IsRootEnabled checks an instance to see if root access is enabled. It returns
// True if root user is enabled for the specified database instance or False
// otherwise.
func IsRootEnabled(client *gophercloud.ServiceClient, id string) (r IsRootEnabledResult) {
	resp, err := client.Get(userRootURL(client, id), &r.Body, nil)
	_, r.Header, r.Err = gophercloud.ParseResponse(resp, err)
	return
}

// Restart will restart only the MySQL Instance. Restarting MySQL will
// erase any dynamic configuration settings that you have made within MySQL.
// The MySQL service will be unavailable until the instance restarts.
func Restart(client *gophercloud.ServiceClient, id string) (r ActionResult) {
	b := map[string]interface{}{"restart": struct{}{}}
	resp, err := client.Post(actionURL(client, id), &b, nil, nil)
	_, r.Header, r.Err = gophercloud.ParseResponse(resp, err)
	return
}

// Resize changes the memory size of the instance, assuming a valid
// flavorRef is provided. It will also restart the MySQL service.
func Resize(client *gophercloud.ServiceClient, id, flavorRef string) (r ActionResult) {
	b := map[string]interface{}{"resize": map[string]string{"flavorRef": flavorRef}}
	resp, err := client.Post(actionURL(client, id), &b, nil, nil)
	_, r.Header, r.Err = gophercloud.ParseResponse(resp, err)
	return
}

// ResizeVolume will resize the attached volume for an instance. It supports
// only increasing the volume size and does not support decreasing the size.
// The volume size is in gigabytes (GB) and must be an integer.
func ResizeVolume(client *gophercloud.ServiceClient, id string, size int) (r ActionResult) {
	b := map[string]interface{}{"resize": map[string]interface{}{"volume": map[string]int{"size": size}}}
	resp, err := client.Post(actionURL(client, id), &b, nil, nil)
	_, r.Header, r.Err = gophercloud.ParseResponse(resp, err)
	return
}

// AttachConfigurationGroup will attach configuration group to the instance
func AttachConfigurationGroup(client *gophercloud.ServiceClient, instanceID string, configID string) (r ConfigurationResult) {
	b := map[string]interface{}{"instance": map[string]interface{}{"configuration": configID}}
	resp, err := client.Put(resourceURL(client, instanceID), &b, nil, &gophercloud.RequestOpts{OkCodes: []int{202}})
	_, r.Header, r.Err = gophercloud.ParseResponse(resp, err)
	return
}

// DetachConfigurationGroup will dettach configuration group from the instance
func DetachConfigurationGroup(client *gophercloud.ServiceClient, instanceID string) (r ConfigurationResult) {
	b := map[string]interface{}{"instance": map[string]interface{}{}}
	resp, err := client.Put(resourceURL(client, instanceID), &b, nil, &gophercloud.RequestOpts{OkCodes: []int{202}})
	_, r.Header, r.Err = gophercloud.ParseResponse(resp, err)
	return
}

// DetachReplica will detach replica from its replication source
func DetachReplica(client *gophercloud.ServiceClient, instanceID string, replicaOf string) (r DetachReplicaResult) {
	b := map[string]interface{}{"instance": map[string]string{"replica_of": replicaOf}}
	resp, err := client.Put(resourceURL(client, instanceID), &b, nil, &gophercloud.RequestOpts{OkCodes: []int{202}})
	_, r.Header, r.Err = gophercloud.ParseResponse(resp, err)
	return
}
