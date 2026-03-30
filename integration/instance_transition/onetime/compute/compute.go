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

// Package compute exposes the cli command for instance migrations.
package compute

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	cpb "github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/protos"
	"github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/usagemetrics"
	"github.com/GoogleCloudPlatform/workloadagentplatform/integration/instance_transition/cmd/persistentflags"
	actions "github.com/GoogleCloudPlatform/workloadagentplatform/integration/instance_transition/usagemetrics"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"
)

type computeOptions struct {
	Project             string
	Zone                string
	Region              string
	Instances           string
	Label               string
	TargetDiskType      string
	TargetInstanceType  string
	TargetZone          string
	TargetSubNetwork    string
	AutoApprove         bool
	RetainName          bool
	DryRun              bool
	Concurrency         int
	DiskConcurrency     int
	Throughput          int
	IOPS                int
	StoragePoolID       string
	Reserve             bool
	ReservationDuration int
	RecreateInstances   bool
	CleanupSnapshots    bool
}

// Compute holds the necessary parameters and integration details for compute instance migration.
type Compute struct {
	Integration *cpb.Integration
	lp          log.Parameters
	cloudProps  *cpb.CloudProperties
	opts        *computeOptions
}

// NewComputeCmd creates a new cobra.Command for compute instance migration.
func NewComputeCmd(ctx context.Context, lp log.Parameters, cloudProps *cpb.CloudProperties, integration *cpb.Integration) *cobra.Command {
	opts := &computeOptions{}
	c := &Compute{
		lp:          lp,
		cloudProps:  cloudProps,
		opts:        opts,
		Integration: integration,
	}

	cmd := &cobra.Command{
		Use:   "compute",
		Short: "Migrate compute instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Execute(cmd)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Project, "project", "p", "", "The Google Cloud project ID where the resources are located")
	flags.StringVar(&opts.Zone, "zone", "", "GCP zone for zonal resources (e.g., us-central1-a)")
	flags.StringVar(&opts.Instances, "instances", "", "Comma-separated instance names or '*' for all")
	flags.StringVar(&opts.Label, "label", "", "Filter by label key=value (e.g., env=staging)")
	flags.StringVar(&opts.TargetInstanceType, "target-instance-type", "", "Target machine type for instance migration (e.g., n2-standard-4)")
	flags.StringVarP(&opts.TargetDiskType, "target-disk-type", "t", "", "Target disk type for migration")
	flags.StringVar(&opts.TargetZone, "target-zone", "", "Target zone for cross-zone migration (e.g., us-central1-b)")
	flags.StringVar(&opts.TargetSubNetwork, "target-subnet", "default", "Subnetwork to use in target zone for cross-zone migration (default: default)")
	flags.BoolVar(&opts.AutoApprove, "auto-approve", false, "Skip all confirmation prompts")
	flags.BoolVar(&opts.RetainName, "retain-name", true, "Reuse original disk name by deleting original (default: true)")
	flags.BoolVar(&opts.DryRun, "dry-run", false, "Preview changes without making them")
	flags.IntVar(&opts.Concurrency, "concurrency", 5, "Number of concurrent instance operations (1-50, default: 5)")
	flags.IntVar(&opts.DiskConcurrency, "disk-concurrency", 0, "Number of concurrent disk operations (1-100, defaults to --concurrency)")
	flags.IntVar(&opts.Throughput, "throughput", 140, "Throughput in MB/s for new disks (140-10000 MB/s)")
	flags.IntVar(&opts.IOPS, "iops", 3000, "IOPS for new disks (3000-500000)")
	flags.StringVarP(&opts.StoragePoolID, "storage-pool-id", "s", "", "Storage pool ID to use for new disks (optional)")
	flags.BoolVar(&opts.Reserve, "reserve", false, "Create shared compute reservation before instance recreation")
	flags.BoolVar(&opts.RecreateInstances, "recreate-instances", false, "Force instance recreation even when in-place update is possible")
	flags.IntVar(&opts.ReservationDuration, "reservation-duration", 2, "Reservation duration in hours (1-24, default: 2)")
	flags.BoolVar(&opts.CleanupSnapshots, "cleanup-snapshots", true, "Automatically delete temporary snapshots after migration (default: true)")

	cmd.MarkFlagRequired("project")
	cmd.MarkFlagRequired("zone")
	cmd.MarkFlagsMutuallyExclusive("instances", "label")

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if opts.Concurrency < 1 || opts.Concurrency > 50 {
			return fmt.Errorf("concurrency must be between 1 and 50")
		}
		if opts.DiskConcurrency < 0 || opts.DiskConcurrency > 100 {
			return fmt.Errorf("disk-concurrency must be between 0 and 100")
		}
		if opts.Throughput < 140 || opts.Throughput > 10000 {
			return fmt.Errorf("throughput must be between 140 and 10000 MB/s")
		}
		if opts.IOPS < 3000 || opts.IOPS > 500000 {
			return fmt.Errorf("iops must be between 3000 and 500000")
		}
		if opts.ReservationDuration < 1 || opts.ReservationDuration > 24 {
			return fmt.Errorf("reservation-duration must be between 1 and 24 hours")
		}
		return nil
	}

	return cmd
}

// Execute runs the compute migration logic for the instance transition.
func (c *Compute) Execute(cmd *cobra.Command) error {
	usagemetrics.SetProperties(c.Integration.GetAgentName(), c.Integration.GetAgentVersion(), true, c.cloudProps)
	usagemetrics.Action(actions.InstanceTransitionComputeStarted)
	persistentflags.SetValues(c.Integration.GetAgentName(), &c.lp, cmd)

	log.SetupLoggingForOTE(c.Integration.GetAgentName(), "compute", c.lp)

	log.Logger.Info("Compute migration executed successfully")

	usagemetrics.Action(actions.InstanceTransitionComputeFinished)
	return nil
}
