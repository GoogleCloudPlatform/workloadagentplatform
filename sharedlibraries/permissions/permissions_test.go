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

package permissions

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var testPermissions = []byte(`
---
features:
  -
    name: HANA_MONITORING
    permissionsList:
      -
        type: Project
        permissions:
          - monitoring.timeSeries.create

  -
    name: PROCESS_METRICS
    permissionsList:
      -
        type: Project
        permissions:
          - compute.nodeGroups.list
          - compute.nodeGroups.get
          - compute.instances.get
          - monitoring.timeSeries.create

  -
    name: CLOUD_LOGGING
    permissionsList:
      -
        type: Project
        permissions:
          - logging.logEntries.create

  -
    name: HOST_METRICS
    permissionsList:
      -
        type: Project
        permissions:
          - compute.instances.list
          - monitoring.metricDescriptors.get
          - monitoring.metricDescriptors.list
      -
        type: Instance
        permissions:
          - compute.instances.get
  -
    name: BACKINT
    permissionsList:
      -
        type: Project
        permissions:
          - storage.objects.list
          - storage.objects.create
      -
        type: Bucket
        permissions:
          - storage.objects.get
          - storage.objects.update
          - storage.objects.delete
  -
    name: BACKINT_MULTIPART
    permissionsList:
      -
        type: Bucket
        permissions:
          - storage.multipartUploads.create
          - storage.multipartUploads.abort

  -
    name: DISKBACKUP
    permissionsList:
      -
        type: Project
        permissions:
          - compute.disks.create
          - compute.disks.createSnapshot
          - compute.disks.setLabels
          - compute.disks.use
          - compute.globalOperations.get
          - compute.instances.attachDisk
          - compute.instances.detachDisk
          - compute.instances.get
          - compute.snapshots.create
          - compute.snapshots.get
          - compute.snapshots.setLabels
          - compute.snapshots.useReadOnly
          - compute.zoneOperations.get
      -
        type: Disk
        permissions:
          - compute.disks.get
  -
    name: DISKBACKUP_STRIPED
    permissionsList:
      -
        type: Project
        permissions:
          - compute.disks.addResourcePolicies
          - compute.disks.create
          - compute.disks.get
          - compute.disks.list
          - compute.disks.removeResourcePolicies
          - compute.disks.use
          - compute.disks.useReadOnly
          - compute.globalOperations.get
          - compute.instances.attachDisk
          - compute.instances.detachDisk
          - compute.instances.get
          - compute.instantSnapshotGroups.create
          - compute.instantSnapshotGroups.delete
          - compute.instantSnapshotGroups.get
          - compute.instantSnapshotGroups.list
          - compute.instantSnapshots.list
          - compute.instantSnapshots.useReadOnly
          - compute.resourcePolicies.create
          - compute.resourcePolicies.use
          - compute.resourcePolicies.useReadOnly
          - compute.snapshots.create
          - compute.snapshots.get
          - compute.snapshots.list
          - compute.snapshots.setLabels
          - compute.snapshots.useReadOnly
          - compute.zoneOperations.get
  -
    name: SAP_SYSTEM_DISCOVERY
    permissionsList:
      -
        type: Project
        permissions:
          - compute.addresses.get
          - compute.addresses.list
          - compute.disks.get
          - compute.forwardingRules.get
          - compute.forwardingRules.list
          - compute.globalAddresses.get
          - compute.globalAddresses.list
          - compute.healthChecks.get
          - compute.instanceGroups.get
          - compute.instances.get
          - compute.instances.list
          - compute.regionBackendServices.get
          - file.instances.get
          - file.instances.list
          - workloadmanager.insights.write
  -
    name: WORKLOAD_EVALUATION_METRICS
    permissionsList:
      -
        type: Project
        permissions:
          - compute.instances.get
          - compute.zoneOperations.list
          - compute.disks.list
          - monitoring.timeSeries.create
          - workloadmanager.insights.write
  -
    name: SECRET_MANAGER
    permissionsList:
      -
        type: Project
        permissions:
          - secretmanager.versions.access
  -
    name: SECRET_MANAGER_SECRET_TYPE
    permissionsList:
      -
        type: Secret
        permissions:
          - secretmanager.versions.access
  -
    name: AGENT_HEALTH_METRICS
    permissionsList:
      -
        type: Project
        permissions:
          - monitoring.timeSeries.create
`)

func stringSliceToMap(s []string) map[string]bool {
	m := make(map[string]bool)
	for _, v := range s {
		m[v] = true
	}
	return m
}

type fakeIAMService struct {
	projectPermissions    map[string][]string
	bucketPermissions     map[string][]string
	diskPermissions       map[string][]string
	instancePermissions   map[string][]string
	secretPermissions     map[string][]string
	projectPermissionErr  error
	bucketPermissionErr   error
	diskPermissionErr     error
	instancePermissionErr error
	secretPermissionErr   error
}

func (f *fakeIAMService) CheckIAMPermissionsOnProject(ctx context.Context, projectID string, permissions []string) ([]string, error) {
	if f.projectPermissionErr != nil {
		return nil, f.projectPermissionErr
	}
	granted := stringSliceToMap(f.projectPermissions[projectID])
	var result []string
	for _, p := range permissions {
		if granted[p] {
			result = append(result, p)
		}
	}
	return result, nil
}

func (f *fakeIAMService) CheckIAMPermissionsOnBucket(ctx context.Context, bucketName string, permissions []string) ([]string, error) {
	if f.bucketPermissionErr != nil {
		return nil, f.bucketPermissionErr
	}
	granted := stringSliceToMap(f.bucketPermissions[bucketName])
	var result []string
	for _, p := range permissions {
		if granted[p] {
			result = append(result, p)
		}
	}
	return result, nil
}

func (f *fakeIAMService) CheckIAMPermissionsOnDisk(ctx context.Context, projectID, zone, diskName string, permissions []string) ([]string, error) {
	if f.diskPermissionErr != nil {
		return nil, f.diskPermissionErr
	}
	key := fmt.Sprintf("%s/%s/%s", projectID, zone, diskName)
	granted := stringSliceToMap(f.diskPermissions[key])
	var result []string
	for _, p := range permissions {
		if granted[p] {
			result = append(result, p)
		}
	}
	return result, nil
}

func (f *fakeIAMService) CheckIAMPermissionsOnInstance(ctx context.Context, projectID, zone, instanceName string, permissions []string) ([]string, error) {
	if f.instancePermissionErr != nil {
		return nil, f.instancePermissionErr
	}
	key := fmt.Sprintf("%s/%s/%s", projectID, zone, instanceName)
	granted := stringSliceToMap(f.instancePermissions[key])
	var result []string
	for _, p := range permissions {
		if granted[p] {
			result = append(result, p)
		}
	}
	return result, nil
}

func (f *fakeIAMService) CheckIAMPermissionsOnSecret(ctx context.Context, projectID, secretName string, permissions []string) ([]string, error) {
	if f.secretPermissionErr != nil {
		return nil, f.secretPermissionErr
	}
	key := fmt.Sprintf("%s/%s", projectID, secretName)
	granted := stringSliceToMap(f.secretPermissions[key])
	var result []string
	for _, p := range permissions {
		if granted[p] {
			result = append(result, p)
		}
	}
	return result, nil
}

func TestNewPermissionsChecker(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pc, err := NewPermissionsChecker(testPermissions)
		if err != nil || pc == nil {
			t.Errorf("NewPermissionsChecker() got %v, %v, want non-nil checker and nil error", pc, err)
		}
	})
	t.Run("failure", func(t *testing.T) {
		badYAML := []byte(`features: [{name: A, permissionsList: [{type: B, permissions: [C]}] BAD`)
		if _, err := NewPermissionsChecker(badYAML); err == nil {
			t.Errorf("NewPermissionsChecker() succeeded with bad YAML, want error")
		}
	})
}

func TestFetchServicePermissionsStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pc, err := NewPermissionsChecker(testPermissions)
	if err != nil {
		t.Fatalf("NewPermissionsChecker failed: %v", err)
	}

	tests := []struct {
		name            string
		serviceName     string
		resourceDetails *ResourceDetails
		iamService      *fakeIAMService
		wantPermissions map[string]bool
		wantErr         error
	}{
		{
			name:        "serviceNotFound",
			serviceName: "invalid-service",
			wantErr:     cmpopts.AnyError,
		},
		{
			name:            "nilResourceDetails",
			serviceName:     "HOST_METRICS",
			resourceDetails: nil,
			wantErr:         cmpopts.AnyError,
		},
		{
			name:        "missingProjectID",
			serviceName: "HOST_METRICS",
			resourceDetails: &ResourceDetails{
				Zone:       "us-central1-a",
				BucketName: "test-bucket",
				DiskName:   "test-disk",
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name:        "missingBucketName",
			serviceName: "BACKINT",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
				Zone:      "us-central1-a",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{
						"compute.nodeGroups.list",
						"compute.nodeGroups.get",
						"compute.instances.get",
						"monitoring.timeSeries.create",
					},
				},
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name:        "missingProjectIDZoneOrDiskName",
			serviceName: "DISKBACKUP",
			resourceDetails: &ResourceDetails{
				Zone:       "us-central1-a",
				BucketName: "test-bucket",
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name:        "missingProjectIDZoneOrInstanceName",
			serviceName: "DISKBACKUP",
			resourceDetails: &ResourceDetails{
				Zone:       "us-central1-a",
				BucketName: "test-bucket",
				DiskName:   "test-disk",
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name:        "missingProjectIDOrSecretName",
			serviceName: "SECRET_MANAGER",
			resourceDetails: &ResourceDetails{
				Zone: "us-central1-a",
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name:        "checkPermissionsFailed",
			serviceName: "HOST_METRICS",
			resourceDetails: &ResourceDetails{
				ProjectID:    "test-project",
				Zone:         "us-central1-a",
				BucketName:   "test-bucket",
				DiskName:     "test-disk",
				InstanceName: "test-instance",
			},
			iamService: &fakeIAMService{
				projectPermissionErr: fmt.Errorf("failed to check permissions"),
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name:        "HOST_METRICS_missingInstanceName",
			serviceName: "HOST_METRICS",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
				Zone:      "us-central1-a",
				// InstanceName is missing
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{
						"compute.instances.list",
						"monitoring.metricDescriptors.get",
						"monitoring.metricDescriptors.list",
					},
				},
			},
			wantErr: cmpopts.AnyError, // Expect an error because InstanceName is missing
		},
		{
			name:        "hanaMonitoring_allPermissionsGranted",
			serviceName: "HANA_MONITORING",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{"monitoring.timeSeries.create"},
				},
			},
			wantPermissions: map[string]bool{
				"monitoring.timeSeries.create": true,
			},
		},
		{
			name:        "hanaMonitoring_somePermissionsNotGranted",
			serviceName: "HANA_MONITORING",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{},
				},
			},
			wantPermissions: map[string]bool{
				"monitoring.timeSeries.create": false,
			},
		},
		{
			name:        "processMetrics_allPermissionsGranted",
			serviceName: "PROCESS_METRICS",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{
						"compute.nodeGroups.list",
						"compute.nodeGroups.get",
						"compute.instances.get",
						"monitoring.timeSeries.create",
					},
				},
			},
			wantPermissions: map[string]bool{
				"compute.nodeGroups.list":      true,
				"compute.nodeGroups.get":       true,
				"compute.instances.get":        true,
				"monitoring.timeSeries.create": true,
			},
		},
		{
			name:        "processMetrics_somePermissionsNotGranted",
			serviceName: "PROCESS_METRICS",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{"compute.nodeGroups.list", "compute.instances.get"},
				},
			},
			wantPermissions: map[string]bool{
				"compute.nodeGroups.list":      true,
				"compute.nodeGroups.get":       false,
				"compute.instances.get":        true,
				"monitoring.timeSeries.create": false,
			},
		},
		{
			name:        "cloudLogging_allPermissionsGranted",
			serviceName: "CLOUD_LOGGING",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{"logging.logEntries.create"},
				},
			},
			wantPermissions: map[string]bool{
				"logging.logEntries.create": true,
			},
		},
		{
			name:        "cloudLogging_somePermissionsNotGranted",
			serviceName: "CLOUD_LOGGING",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{},
				},
			},
			wantPermissions: map[string]bool{
				"logging.logEntries.create": false,
			},
		},
		{
			name:        "hostMetrics_allPermissionsGranted",
			serviceName: "HOST_METRICS",
			resourceDetails: &ResourceDetails{
				ProjectID:    "test-project",
				Zone:         "us-central1-a",
				BucketName:   "test-bucket",
				DiskName:     "test-disk",
				InstanceName: "test-instance",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{
						"compute.instances.list",
						"monitoring.metricDescriptors.get",
						"monitoring.metricDescriptors.list",
					},
				},
				instancePermissions: map[string][]string{
					"test-project/us-central1-a/test-instance": []string{"compute.instances.get"},
				},
			},
			wantPermissions: map[string]bool{
				"compute.instances.list":            true,
				"compute.instances.get":             true,
				"monitoring.metricDescriptors.get":  true,
				"monitoring.metricDescriptors.list": true,
			},
		},
		{
			name:        "hostMetrics_somePermissionsNotGranted",
			serviceName: "HOST_METRICS",
			resourceDetails: &ResourceDetails{
				ProjectID:    "test-project",
				Zone:         "us-central1-a",
				BucketName:   "test-bucket",
				DiskName:     "test-disk",
				InstanceName: "test-instance",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{"compute.instances.list"},
				},
				instancePermissions: map[string][]string{
					"test-project/us-central1-a/test-instance": []string{"compute.instances.get"},
				},
			},
			wantPermissions: map[string]bool{
				"compute.instances.list":            true,
				"monitoring.metricDescriptors.get":  false,
				"monitoring.metricDescriptors.list": false,
				"compute.instances.get":             true,
			},
		},
		{
			name:        "backint_allPermissionsGranted",
			serviceName: "BACKINT",
			resourceDetails: &ResourceDetails{
				ProjectID:  "test-project",
				BucketName: "test-bucket",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{
						"storage.objects.list",
						"storage.objects.create",
					},
				},
				bucketPermissions: map[string][]string{
					"test-bucket": []string{
						"storage.objects.get",
						"storage.objects.update",
						"storage.objects.delete",
					},
				},
			},
			wantPermissions: map[string]bool{
				"storage.objects.list":   true,
				"storage.objects.create": true,
				"storage.objects.get":    true,
				"storage.objects.update": true,
				"storage.objects.delete": true,
			},
		},
		{
			name:        "backint_somePermissionsNotGranted",
			serviceName: "BACKINT",
			resourceDetails: &ResourceDetails{
				ProjectID:  "test-project",
				BucketName: "test-bucket",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{"storage.objects.list"},
				},
				bucketPermissions: map[string][]string{
					"test-bucket": []string{
						"storage.objects.get",
						"storage.objects.update",
					},
				},
			},
			wantPermissions: map[string]bool{
				"storage.objects.list":   true,
				"storage.objects.create": false,
				"storage.objects.get":    true,
				"storage.objects.update": true,
				"storage.objects.delete": false,
			},
		},
		{
			name:        "backintMultipart_allPermissionsGranted",
			serviceName: "BACKINT_MULTIPART",
			resourceDetails: &ResourceDetails{
				BucketName: "test-bucket",
			},
			iamService: &fakeIAMService{
				bucketPermissions: map[string][]string{
					"test-bucket": []string{
						"storage.multipartUploads.create",
						"storage.multipartUploads.abort",
					},
				},
			},
			wantPermissions: map[string]bool{
				"storage.multipartUploads.create": true,
				"storage.multipartUploads.abort":  true,
			},
		},
		{
			name:        "backintMultipart_somePermissionsNotGranted",
			serviceName: "BACKINT_MULTIPART",
			resourceDetails: &ResourceDetails{
				BucketName: "test-bucket",
			},
			iamService: &fakeIAMService{
				bucketPermissions: map[string][]string{
					"test-bucket": []string{"storage.multipartUploads.create"},
				},
			},
			wantPermissions: map[string]bool{
				"storage.multipartUploads.create": true,
				"storage.multipartUploads.abort":  false,
			},
		},
		{
			name:        "diskbackup_allPermissionsGranted",
			serviceName: "DISKBACKUP",
			resourceDetails: &ResourceDetails{
				ProjectID:    "test-project",
				Zone:         "us-central1-a",
				DiskName:     "test-disk",
				InstanceName: "test-instance",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{
						"compute.disks.create",
						"compute.disks.createSnapshot",
						"compute.disks.setLabels",
						"compute.disks.use",
						"compute.globalOperations.get",
						"compute.instances.attachDisk",
						"compute.instances.detachDisk",
						"compute.instances.get",
						"compute.snapshots.create",
						"compute.snapshots.get",
						"compute.snapshots.setLabels",
						"compute.snapshots.useReadOnly",
						"compute.zoneOperations.get",
					},
				},
				diskPermissions: map[string][]string{
					"test-project/us-central1-a/test-disk": []string{"compute.disks.get"},
				},
				instancePermissions: map[string][]string{
					"test-project/us-central1-a/test-instance": []string{"compute.instances.get"},
				},
			},
			wantPermissions: map[string]bool{
				"compute.disks.create":          true,
				"compute.disks.createSnapshot":  true,
				"compute.disks.get":             true,
				"compute.disks.setLabels":       true,
				"compute.disks.use":             true,
				"compute.globalOperations.get":  true,
				"compute.instances.attachDisk":  true,
				"compute.instances.detachDisk":  true,
				"compute.instances.get":         true,
				"compute.snapshots.create":      true,
				"compute.snapshots.get":         true,
				"compute.snapshots.setLabels":   true,
				"compute.snapshots.useReadOnly": true,
				"compute.zoneOperations.get":    true,
			},
		},
		{
			name:        "diskbackup_somePermissionsNotGranted",
			serviceName: "DISKBACKUP",
			resourceDetails: &ResourceDetails{
				ProjectID:    "test-project",
				Zone:         "us-central1-a",
				DiskName:     "test-disk",
				InstanceName: "test-instance",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{
						"compute.disks.createSnapshot",
						"compute.disks.setLabels",
						"compute.disks.use",
						"compute.globalOperations.get",
						"compute.instances.attachDisk",
						"compute.instances.detachDisk",
						"compute.instances.get",
						"compute.snapshots.create",
						"compute.snapshots.get",
						"compute.snapshots.setLabels",
						"compute.snapshots.useReadOnly",
						"compute.zoneOperations.get",
					},
				},
				diskPermissions: map[string][]string{
					"test-project/us-central1-a/test-disk": []string{},
				},
				instancePermissions: map[string][]string{
					"test-project/us-central1-a/test-instance": []string{"compute.instances.get"},
				},
			},
			wantPermissions: map[string]bool{
				"compute.disks.create":          false,
				"compute.disks.createSnapshot":  true,
				"compute.disks.get":             false,
				"compute.disks.setLabels":       true,
				"compute.disks.use":             true,
				"compute.globalOperations.get":  true,
				"compute.instances.attachDisk":  true,
				"compute.instances.detachDisk":  true,
				"compute.instances.get":         true,
				"compute.snapshots.create":      true,
				"compute.snapshots.get":         true,
				"compute.snapshots.setLabels":   true,
				"compute.snapshots.useReadOnly": true,
				"compute.zoneOperations.get":    true,
			},
		},
		{
			name:        "diskbackupStriped_allPermissionsGranted",
			serviceName: "DISKBACKUP_STRIPED",
			resourceDetails: &ResourceDetails{
				ProjectID:    "test-project",
				Zone:         "us-central1-a",
				DiskName:     "test-disk",
				InstanceName: "test-instance",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{
						"compute.disks.addResourcePolicies",
						"compute.disks.create",
						"compute.disks.get",
						"compute.disks.list",
						"compute.disks.removeResourcePolicies",
						"compute.disks.use",
						"compute.disks.useReadOnly",
						"compute.globalOperations.get",
						"compute.instances.attachDisk",
						"compute.instances.detachDisk",
						"compute.instances.get",
						"compute.instantSnapshotGroups.create",
						"compute.instantSnapshotGroups.delete",
						"compute.instantSnapshotGroups.get",
						"compute.instantSnapshotGroups.list",
						"compute.instantSnapshots.list",
						"compute.instantSnapshots.useReadOnly",
						"compute.resourcePolicies.create",
						"compute.resourcePolicies.use",
						"compute.resourcePolicies.useReadOnly",
						"compute.snapshots.create",
						"compute.snapshots.get",
						"compute.snapshots.list",
						"compute.snapshots.setLabels",
						"compute.snapshots.useReadOnly",
						"compute.zoneOperations.get",
					},
				},
				diskPermissions: map[string][]string{
					"test-project/us-central1-a/test-disk": []string{"compute.disks.get"},
				},
				instancePermissions: map[string][]string{
					"test-project/us-central1-a/test-instance": []string{"compute.instances.get"},
				},
			},
			wantPermissions: map[string]bool{
				"compute.disks.addResourcePolicies":    true,
				"compute.disks.create":                 true,
				"compute.disks.get":                    true,
				"compute.disks.list":                   true,
				"compute.disks.removeResourcePolicies": true,
				"compute.disks.use":                    true,
				"compute.disks.useReadOnly":            true,
				"compute.globalOperations.get":         true,
				"compute.instances.attachDisk":         true,
				"compute.instances.detachDisk":         true,
				"compute.instances.get":                true,
				"compute.instantSnapshotGroups.create": true,
				"compute.instantSnapshotGroups.delete": true,
				"compute.instantSnapshotGroups.get":    true,
				"compute.instantSnapshotGroups.list":   true,
				"compute.instantSnapshots.list":        true,
				"compute.instantSnapshots.useReadOnly": true,
				"compute.resourcePolicies.create":      true,
				"compute.resourcePolicies.use":         true,
				"compute.resourcePolicies.useReadOnly": true,
				"compute.snapshots.create":             true,
				"compute.snapshots.get":                true,
				"compute.snapshots.list":               true,
				"compute.snapshots.setLabels":          true,
				"compute.snapshots.useReadOnly":        true,
				"compute.zoneOperations.get":           true,
			},
		},
		{
			name:        "diskbackupStriped_somePermissionsNotGranted",
			serviceName: "DISKBACKUP_STRIPED",
			resourceDetails: &ResourceDetails{
				ProjectID:    "test-project",
				Zone:         "us-central1-a",
				DiskName:     "test-disk",
				InstanceName: "test-instance",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{
						"compute.disks.create",
						"compute.disks.get",
						"compute.disks.list",
						"compute.disks.removeResourcePolicies",
						"compute.disks.use",
						"compute.disks.useReadOnly",
						"compute.globalOperations.get",
						"compute.instances.attachDisk",
						"compute.instances.detachDisk",
						"compute.instances.get",
						"compute.instantSnapshotGroups.create",
						"compute.instantSnapshotGroups.delete",
						"compute.instantSnapshotGroups.get",
						"compute.instantSnapshotGroups.list",
						"compute.instantSnapshots.list",
						"compute.instantSnapshots.useReadOnly",
						"compute.resourcePolicies.create",
						"compute.resourcePolicies.use",
						"compute.resourcePolicies.useReadOnly",
						"compute.snapshots.create",
						"compute.snapshots.get",
						"compute.snapshots.list",
						"compute.snapshots.setLabels",
						"compute.snapshots.useReadOnly",
						"compute.zoneOperations.get",
					},
				},
				diskPermissions: map[string][]string{
					"test-project/us-central1-a/test-disk": []string{},
				},
				instancePermissions: map[string][]string{
					"test-project/us-central1-a/test-instance": []string{"compute.instances.get"},
				},
			},
			wantPermissions: map[string]bool{
				"compute.disks.addResourcePolicies":    false,
				"compute.disks.create":                 true,
				"compute.disks.get":                    true,
				"compute.disks.list":                   true,
				"compute.disks.removeResourcePolicies": true,
				"compute.disks.use":                    true,
				"compute.disks.useReadOnly":            true,
				"compute.globalOperations.get":         true,
				"compute.instances.attachDisk":         true,
				"compute.instances.detachDisk":         true,
				"compute.instances.get":                true,
				"compute.instantSnapshotGroups.create": true,
				"compute.instantSnapshotGroups.delete": true,
				"compute.instantSnapshotGroups.get":    true,
				"compute.instantSnapshotGroups.list":   true,
				"compute.instantSnapshots.list":        true,
				"compute.instantSnapshots.useReadOnly": true,
				"compute.resourcePolicies.create":      true,
				"compute.resourcePolicies.use":         true,
				"compute.resourcePolicies.useReadOnly": true,
				"compute.snapshots.create":             true,
				"compute.snapshots.get":                true,
				"compute.snapshots.list":               true,
				"compute.snapshots.setLabels":          true,
				"compute.snapshots.useReadOnly":        true,
				"compute.zoneOperations.get":           true,
			},
		},
		{
			name:        "sapSystemDiscovery_allPermissionsGranted",
			serviceName: "SAP_SYSTEM_DISCOVERY",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{
						"compute.addresses.get",
						"compute.addresses.list",
						"compute.disks.get",
						"compute.forwardingRules.get",
						"compute.forwardingRules.list",
						"compute.globalAddresses.get",
						"compute.globalAddresses.list",
						"compute.healthChecks.get",
						"compute.instanceGroups.get",
						"compute.instances.get",
						"compute.instances.list",
						"compute.regionBackendServices.get",
						"file.instances.get",
						"file.instances.list",
						"workloadmanager.insights.write",
					},
				},
			},
			wantPermissions: map[string]bool{
				"compute.addresses.get":             true,
				"compute.addresses.list":            true,
				"compute.disks.get":                 true,
				"compute.forwardingRules.get":       true,
				"compute.forwardingRules.list":      true,
				"compute.globalAddresses.get":       true,
				"compute.globalAddresses.list":      true,
				"compute.healthChecks.get":          true,
				"compute.instanceGroups.get":        true,
				"compute.instances.get":             true,
				"compute.instances.list":            true,
				"compute.regionBackendServices.get": true,
				"file.instances.get":                true,
				"file.instances.list":               true,
				"workloadmanager.insights.write":    true,
			},
		},
		{
			name:        "sapSystemDiscovery_somePermissionsNotGranted",
			serviceName: "SAP_SYSTEM_DISCOVERY",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{"compute.addresses.get"},
				},
			},
			wantPermissions: map[string]bool{
				"compute.addresses.get":             true,
				"compute.addresses.list":            false,
				"compute.disks.get":                 false,
				"compute.forwardingRules.get":       false,
				"compute.forwardingRules.list":      false,
				"compute.globalAddresses.get":       false,
				"compute.globalAddresses.list":      false,
				"compute.healthChecks.get":          false,
				"compute.instanceGroups.get":        false,
				"compute.instances.get":             false,
				"compute.instances.list":            false,
				"compute.regionBackendServices.get": false,
				"file.instances.get":                false,
				"file.instances.list":               false,
				"workloadmanager.insights.write":    false,
			},
		},
		{
			name:        "workloadEvaluationMetrics_allPermissionsGranted",
			serviceName: "WORKLOAD_EVALUATION_METRICS",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{
						"compute.instances.get",
						"compute.zoneOperations.list",
						"compute.disks.list",
						"monitoring.timeSeries.create",
						"workloadmanager.insights.write",
					},
				},
			},
			wantPermissions: map[string]bool{
				"compute.instances.get":          true,
				"compute.zoneOperations.list":    true,
				"compute.disks.list":             true,
				"monitoring.timeSeries.create":   true,
				"workloadmanager.insights.write": true,
			},
		},
		{
			name:        "workloadEvaluationMetrics_somePermissionsNotGranted",
			serviceName: "WORKLOAD_EVALUATION_METRICS",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{"compute.instances.get"},
				},
			},
			wantPermissions: map[string]bool{
				"compute.instances.get":          true,
				"compute.zoneOperations.list":    false,
				"compute.disks.list":             false,
				"monitoring.timeSeries.create":   false,
				"workloadmanager.insights.write": false,
			},
		},
		{
			name:        "secretManager_allPermissionsGranted",
			serviceName: "SECRET_MANAGER",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{"secretmanager.versions.access"},
				},
			},
			wantPermissions: map[string]bool{
				"secretmanager.versions.access": true,
			},
		},
		{
			name:        "secretManager_somePermissionsNotGranted",
			serviceName: "SECRET_MANAGER",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{},
				},
			},
			wantPermissions: map[string]bool{
				"secretmanager.versions.access": false,
			},
		},
		{
			name:        "secretManagerSecretType_allPermissionsGranted",
			serviceName: "SECRET_MANAGER_SECRET_TYPE",
			resourceDetails: &ResourceDetails{
				ProjectID:  "test-project",
				SecretName: "test-secret",
			},
			iamService: &fakeIAMService{
				secretPermissions: map[string][]string{
					"test-project/test-secret": []string{"secretmanager.versions.access"},
				},
			},
			wantPermissions: map[string]bool{
				"secretmanager.versions.access": true,
			},
		},
		{
			name:        "secretManagerSecretType_somePermissionsNotGranted",
			serviceName: "SECRET_MANAGER_SECRET_TYPE",
			resourceDetails: &ResourceDetails{
				ProjectID:  "test-project",
				SecretName: "test-secret",
			},
			iamService: &fakeIAMService{
				secretPermissions: map[string][]string{
					"test-project/test-secret": []string{},
				},
			},
			wantPermissions: map[string]bool{
				"secretmanager.versions.access": false,
			},
		},
		{
			name:        "secretManagerSecretType_missingSecretName",
			serviceName: "SECRET_MANAGER_SECRET_TYPE",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
			},
			iamService: &fakeIAMService{},
			wantErr:    cmpopts.AnyError,
		},
		{
			name:        "agentHealthMetrics_allPermissionsGranted",
			serviceName: "AGENT_HEALTH_METRICS",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{"monitoring.timeSeries.create"},
				},
			},
			wantPermissions: map[string]bool{
				"monitoring.timeSeries.create": true,
			},
		},
		{
			name:        "agentHealthMetrics_somePermissionsNotGranted",
			serviceName: "AGENT_HEALTH_METRICS",
			resourceDetails: &ResourceDetails{
				ProjectID: "test-project",
			},
			iamService: &fakeIAMService{
				projectPermissions: map[string][]string{
					"test-project": []string{},
				},
			},
			wantPermissions: map[string]bool{
				"monitoring.timeSeries.create": false,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotPermissions, gotErr := pc.FetchServicePermissionsStatus(ctx, tc.iamService, tc.serviceName, tc.resourceDetails)
			if !cmp.Equal(gotErr, tc.wantErr, cmpopts.EquateErrors()) {
				t.Errorf("FetchServicePermissionsStatus(service=%s, details=%+v) got error: %v, want error: %v", tc.serviceName, tc.resourceDetails, gotErr, tc.wantErr)
			}
			if tc.wantErr == nil {
				if diff := cmp.Diff(tc.wantPermissions, gotPermissions); diff != "" {
					t.Errorf("FetchServicePermissionsStatus(service=%s, details=%+v) returned unexpected diff (-want +got):\n%s", tc.serviceName, tc.resourceDetails, diff)
				}
			}
		})
	}
}
