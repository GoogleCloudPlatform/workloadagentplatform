/*
Copyright 2024 Google LLC

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

package compute

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
	cpb "github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/protos"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"
)

func TestExecute(t *testing.T) {
	tests := []struct {
		name string
		c    *Compute
		want error
	}{
		{
			name: "Success",
			c: &Compute{
				Integration: &cpb.Integration{
					AgentName:       "test-agent",
					AgentVersion:    "1.0.0",
					AgentBinaryName: "test-binary",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.c.Execute(&cobra.Command{})
			if got != test.want {
				t.Errorf("Execute(%v) = %v, want %v", test.c, got, test.want)
			}
		})
	}
}

func TestNewComputeHelp(t *testing.T) {
	cmd := NewComputeCmd(context.Background(), log.Parameters{}, &cpb.CloudProperties{}, &computeOptions{}, &cpb.Integration{})
	got := cmd.Short
	want := "Migrate compute instances"
	if got != want {
		t.Errorf("Short description = %q, want %q", got, want)
	}
}

func TestComputeFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    *computeOptions
		wantErr bool
	}{
		{
			name: "AllFlagsInstances",
			args: []string{
				"--project", "test-project",
				"--zone", "us-central1-a",
				"--instances", "instance-1,instance-2",
				"-t", "pd-standard",
				"--target-instance-type", "n2-standard-4",
				"--target-zone", "us-central1-b",
				"--target-subnet", "my-subnet",
				"--auto-approve",
				"--retain-name=false",
				"--dry-run",
				"--concurrency", "10",
				"--disk-concurrency", "20",
				"--throughput", "200",
				"--iops", "5000",
				"-s", "pool-1",
				"--reserve",
				"--recreate-instances",
				"--reservation-duration", "5",
				"--cleanup-snapshots=false",
			},
			want: &computeOptions{
				Project:             "test-project",
				Zone:                "us-central1-a",
				Instances:           "instance-1,instance-2",
				TargetDiskType:      "pd-standard",
				TargetInstanceType:  "n2-standard-4",
				TargetZone:          "us-central1-b",
				TargetSubNetwork:    "my-subnet",
				AutoApprove:         true,
				RetainName:          false,
				DryRun:              true,
				Concurrency:         10,
				DiskConcurrency:     20,
				Throughput:          200,
				IOPS:                5000,
				StoragePoolID:       "pool-1",
				Reserve:             true,
				RecreateInstances:   true,
				ReservationDuration: 5,
				CleanupSnapshots:    false,
			},
		},
		{
			name: "AllFlagsLabel",
			args: []string{
				"--project", "test-project",
				"--label", "env=prod",
			},
			want: &computeOptions{
				Project:             "test-project",
				Label:               "env=prod",
				RetainName:          true,
				TargetSubNetwork:    "default",
				Concurrency:         5,
				IOPS:                3000,
				Throughput:          140,
				ReservationDuration: 2,
				CleanupSnapshots:    true,
			},
		},
		{
			name: "MissingProject",
			args: []string{
				"--instances", "instance-1",
			},
			wantErr: true,
		},
		{
			name: "MutuallyExclusiveInstancesAndLabel",
			args: []string{
				"--project", "test-project",
				"--instances", "instance-1",
				"--label", "env=prod",
			},
			wantErr: true,
		},
		{
			name: "InvalidConcurrency",
			args: []string{
				"--project", "test-project",
				"--concurrency", "100",
			},
			wantErr: true,
		},
		{
			name: "InvalidIOPS",
			args: []string{
				"--project", "test-project",
				"--iops", "1000",
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opts := &computeOptions{}
			cmd := NewComputeCmd(context.Background(), log.Parameters{}, &cpb.CloudProperties{}, opts, &cpb.Integration{
				AgentName:       "test-agent",
				AgentVersion:    "1.0.0",
				AgentBinaryName: "test-binary",
			})

			cmd.SetArgs(test.args)
			err := cmd.Execute()
			if (err != nil) != test.wantErr {
				t.Fatalf("Execute() error = %v, wantErr %v", err, test.wantErr)
			}

			if test.wantErr {
				return
			}

			if diff := cmp.Diff(test.want, opts); diff != "" {
				t.Errorf("opts diff (-want +got):\n%s", diff)
			}
		})
	}
}
