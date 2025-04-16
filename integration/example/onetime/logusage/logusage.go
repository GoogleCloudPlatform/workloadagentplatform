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

// Package logusage implements the one time execution mode for usage logging.
package logusage

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/usagemetrics"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"

	cpb "github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/protos"
)

// LogUsage has args for logusage subcommands.
type LogUsage struct {
	name, agentVersion, status, image string
	action, usageError                int
	lp                                log.Parameters
}

// logUsageStatus makes a call to the appropriate usage metrics API.
func (l *LogUsage) logUsageStatus(cloudProps *cpb.CloudProperties) error {
	configureUsageMetricsForOTE(cloudProps, l.name, l.agentVersion, l.image)
	switch usagemetrics.ParseStatus(l.status) {
	case usagemetrics.StatusRunning:
		usagemetrics.Running()
	case usagemetrics.StatusStarted:
		usagemetrics.Started()
	case usagemetrics.StatusStopped:
		usagemetrics.Stopped()
	case usagemetrics.StatusConfigured:
		usagemetrics.Configured()
	case usagemetrics.StatusMisconfigured:
		usagemetrics.Misconfigured()
	case usagemetrics.StatusError:
		usagemetrics.Error(l.usageError)
	case usagemetrics.StatusInstalled:
		usagemetrics.Installed()
	case usagemetrics.StatusUpdated:
		usagemetrics.Updated(l.agentVersion)
	case usagemetrics.StatusUninstalled:
		usagemetrics.Uninstalled()
	case usagemetrics.StatusAction:
		usagemetrics.Action(l.action)
	default:
		return fmt.Errorf("logUsageStatus() called with an unknown status: %s", l.status)
	}
	return nil
}

func configureUsageMetricsForOTE(cp *cpb.CloudProperties, name, version, image string) {
	// Override the imageURL with value passed in args.
	if image != "" && cp != nil {
		cp.Image = image
	}
	usagemetrics.SetProperties(name, version, true, cp)
}

// NewLogUsageCommand creates a new logusage command.
func NewLogUsageCommand(ctx context.Context, lp log.Parameters, cloudProps *cpb.CloudProperties, integration *cpb.Integration) *cobra.Command {
	l := &LogUsage{lp: lp}
	logUsageCmd := &cobra.Command{
		Use:   `logusage`,
		Short: "Log usage metrics for the agent",
		Long:  "Usage: logusage [--name <tool or agent name>] [--av <tool or agent version>] [--status <RUNNING|INSTALLED|...>] [--action <integer action code>] [--error <integer error code>] [--image <image URL of the compute instance>] [-v] [-h]",
		RunE: func(cmd *cobra.Command, args []string) error {
			return l.logUsageHandler(cmd, cloudProps)
		},
	}

	logUsageCmd.Flags().StringVarP(&l.name, "name", "n", integration.GetAgentName(), "Agent or Tool name")
	logUsageCmd.Flags().StringVar(&l.agentVersion, "agent-version", integration.GetAgentVersion(), "Agent or Tool version")
	logUsageCmd.Flags().StringVar(&l.agentVersion, "av", integration.GetAgentVersion(), "Agent or Tool version")
	logUsageCmd.Flags().StringVarP(&l.status, "status", "s", "", "usage status value")
	logUsageCmd.Flags().IntVarP(&l.action, "action", "a", 0, "usage action code")
	logUsageCmd.Flags().IntVarP(&l.usageError, "error", "e", 0, "usage error code")
	logUsageCmd.Flags().StringVarP(&l.image, "image", "i", "", "the image url of the compute instance(optional), default value is retrieved from metadata)")

	return logUsageCmd
}

func (l *LogUsage) logUsageHandler(cmd *cobra.Command, cloudProps *cpb.CloudProperties) error {
	if l.lp.CloudLoggingClient != nil {
		defer l.lp.CloudLoggingClient.Close()
	}
	log.SetupLoggingForOTE(l.name, "logusage", l.lp)
	switch {
	case l.status == "":
		log.Print("A usage status value is required.")
		return fmt.Errorf("a usage status value is required")
	case l.status == string(usagemetrics.StatusUpdated) && l.agentVersion == "":
		log.Print("For status UPDATED, Agent Version is required.")
		return fmt.Errorf("for status UPDATED, Agent Version is required")
	case l.status == string(usagemetrics.StatusError) && l.usageError <= 0:
		log.Print("For status ERROR, an error code is required.")
		return fmt.Errorf("for status ERROR, an error code is required")
	case l.status == string(usagemetrics.StatusAction) && l.action <= 0:
		log.Print("For status ACTION, an action code is required.")
		return fmt.Errorf("for status ACTION, an action code is required")
	}

	if err := l.logUsageStatus(cloudProps); err != nil {
		log.Logger.Warnw("Could not log usage", "error", err)
	}
	return nil
}
