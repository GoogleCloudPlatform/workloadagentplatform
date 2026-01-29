/*
Copyright 2026 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package permissions provides functions to check IAM permissions for workload agent platform services.
package permissions

import (
	"context"
	"fmt"

	"github.com/go-yaml/yaml"
)

type (
	yamlFeatures struct {
		Features []servicePermissions `yaml:"features"`
	}

	// servicePermissions is a struct to hold the permissions for a service.
	servicePermissions struct {
		Name            string              `yaml:"name"`
		PermissionsList []EntityPermissions `yaml:"permissionsList"`
	}

	// EntityPermissions is a struct to hold the permissions for an entity.
	EntityPermissions struct {
		Type        string   `yaml:"type"`
		Permissions []string `yaml:"permissions"`
	}

	// ResourceDetails is a struct to hold the details of the resources
	// (project/disk etc) on which the permissions are checked.
	ResourceDetails struct {
		ProjectID    string
		Zone         string
		BucketName   string
		DiskName     string
		InstanceName string
		SecretName   string
	}

	// IAMService is an interface for an IAM service.
	IAMService interface {
		CheckIAMPermissionsOnProject(ctx context.Context, projectID string, permissions []string) ([]string, error)
		CheckIAMPermissionsOnBucket(ctx context.Context, bucketName string, permissions []string) ([]string, error)
		CheckIAMPermissionsOnDisk(ctx context.Context, projectID, zone, diskName string, permissions []string) ([]string, error)
		CheckIAMPermissionsOnInstance(ctx context.Context, projectID, zone, instanceName string, permissions []string) ([]string, error)
		CheckIAMPermissionsOnSecret(ctx context.Context, projectID, secretName string, permissions []string) ([]string, error)
	}
)

// Checker holds the parsed permissions configuration.
type Checker struct {
	permissionsMap map[string][]EntityPermissions
}

const (
	// ProjectResourceType is the resource type for Project.
	ProjectResourceType = "Project"
	// BucketResourceType is the resource type for Bucket.
	BucketResourceType = "Bucket"
	// DiskResourceType is the resource type for Disk.
	DiskResourceType = "Disk"
	// InstanceResourceType is the resource type for Instance.
	InstanceResourceType = "Instance"
	// SecretResourceType is the resource type for Secret.
	SecretResourceType = "Secret"
)

// NewPermissionsChecker parses the YAML data and returns a new Checker.
func NewPermissionsChecker(iamPermissionsYAML []byte) (*Checker, error) {
	var permissionsData yamlFeatures
	err := yaml.Unmarshal(iamPermissionsYAML, &permissionsData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %v", err)
	}

	pMap := make(map[string][]EntityPermissions)
	for _, service := range permissionsData.Features {
		pMap[service.Name] = service.PermissionsList
	}
	return &Checker{permissionsMap: pMap}, nil
}

// FetchServicePermissionsStatus checks if the required IAM permissions for a
// service/functionality are granted on the specified resource, and returns
// a map of permissions to granted/not granted. Assumes that the permissions
// are unique across all resource types for a service.
func (pc *Checker) FetchServicePermissionsStatus(ctx context.Context, iamService IAMService, serviceName string, resDetails *ResourceDetails) (map[string]bool, error) {
	if pc == nil || pc.permissionsMap == nil {
		return nil, fmt.Errorf("checker is not initialized")
	}
	if resDetails == nil {
		return nil, fmt.Errorf("resourceDetails cannot be nil")
	}
	// Get the permissions for the service from the permissionsMap
	permissionsList, ok := pc.permissionsMap[serviceName]
	if !ok {
		return nil, fmt.Errorf("service not found: %s", serviceName)
	}

	allPermissions := newPermissionsStatusMap(permissionsList)
	// Check permissions for each resource type in the permissionsList
	var allGrantedPermissions []string
	for _, permList := range permissionsList {
		// Call the appropriate IAM check function based on resource type
		var grantedPermissions []string
		var err error
		switch permList.Type {
		case ProjectResourceType:
			if resDetails.ProjectID == "" {
				return nil, fmt.Errorf("missing ProjectID in ResourceDetails")
			}
			grantedPermissions, err = iamService.CheckIAMPermissionsOnProject(ctx, resDetails.ProjectID, permList.Permissions)
		case BucketResourceType:
			if resDetails.BucketName == "" {
				return nil, fmt.Errorf("missing BucketName in ResourceDetails")
			}
			grantedPermissions, err = iamService.CheckIAMPermissionsOnBucket(ctx, resDetails.BucketName, permList.Permissions)
		case DiskResourceType:
			if resDetails.ProjectID == "" || resDetails.Zone == "" || resDetails.DiskName == "" {
				return nil, fmt.Errorf("missing ProjectID, Zone, or DiskName in ResourceDetails")
			}
			grantedPermissions, err = iamService.CheckIAMPermissionsOnDisk(ctx, resDetails.ProjectID, resDetails.Zone, resDetails.DiskName, permList.Permissions)
		case InstanceResourceType:
			if resDetails.ProjectID == "" || resDetails.Zone == "" || resDetails.InstanceName == "" {
				return nil, fmt.Errorf("missing ProjectID, Zone, or InstanceName in ResourceDetails")
			}
			grantedPermissions, err = iamService.CheckIAMPermissionsOnInstance(ctx, resDetails.ProjectID, resDetails.Zone, resDetails.InstanceName, permList.Permissions)
		case SecretResourceType:
			if resDetails.ProjectID == "" || resDetails.SecretName == "" {
				return nil, fmt.Errorf("missing ProjectID or SecretName in ResourceDetails")
			}
			grantedPermissions, err = iamService.CheckIAMPermissionsOnSecret(ctx, resDetails.ProjectID, resDetails.SecretName, permList.Permissions)
		default:
			return nil, fmt.Errorf("unsupported resource type: %s", permList.Type)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to check permissions for service %s on entity %s: %v", serviceName, permList.Type, err)
		}

		allGrantedPermissions = append(allGrantedPermissions, grantedPermissions...)
	}

	// Sets the permissions map to true for all granted permissions.
	// The final `allPermissions` map keys are unique. If a permission is listed for multiple resource types
	// for the same service, it will be marked `true` if granted on any of those resources.
	for _, perm := range allGrantedPermissions {
		allPermissions[perm] = true
	}

	return allPermissions, nil
}

// newPermissionsStatusMap creates a map of permissions defaulted to false (i.e.
// not granted).
func newPermissionsStatusMap(permissionsList []EntityPermissions) map[string]bool {
	allPermissions := make(map[string]bool)
	for _, permList := range permissionsList {
		for _, perm := range permList.Permissions {
			allPermissions[perm] = false
		}
	}

	return allPermissions
}
