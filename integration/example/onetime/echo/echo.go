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

// Package echo implements the one time execution mode for displaying a value to the logs.
package echo

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	cpb "github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/protos"
	"github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/usagemetrics"
	"github.com/GoogleCloudPlatform/workloadagentplatform/integration/example/cmd/persistentflags"
	actions "github.com/GoogleCloudPlatform/workloadagentplatform/integration/example/usagemetrics"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"
)

// Name of the OTE subcommand.
var oteName = "echo"

// Echo has args for echo subcommands.
type Echo struct {
	text        string
	Integration *cpb.Integration
	lp          log.Parameters
	cloudProps  *cpb.CloudProperties
}

// NewEcho creates a new echo cobra command.
func NewEcho(ctx context.Context, lp log.Parameters, cloudProps *cpb.CloudProperties, integration *cpb.Integration) *cobra.Command {
	e := &Echo{
		lp:          lp,
		cloudProps:  cloudProps,
		Integration: integration,
	}
	echoCmd := &cobra.Command{
		Use:   oteName,
		Short: "Prints text to the logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.Execute(cmd)
		},
	}
	echoCmd.Flags().StringVar(&e.text, "text", "", "text to print to the logs")
	echoCmd.SetContext(ctx)
	return echoCmd
}

// Execute prints the text to standard out, OTE logs, and Cloud Logging.
func (e *Echo) Execute(cmd *cobra.Command) error {
	// Set the usage metrics properties and register the action with usage metrics.
	usagemetrics.SetProperties(e.Integration.GetAgentName(), e.Integration.GetAgentVersion(), true, e.cloudProps)
	usagemetrics.Action(actions.ExampleEchoStarted)

	// No text argument means no logging is necessary.
	if e.text == "" {
		return nil
	}

	persistentflags.SetValues(e.Integration.GetAgentName(), &e.lp, cmd)
	if e.lp.CloudLoggingClient != nil {
		defer e.lp.CloudLoggingClient.Close()
	}
	log.SetupLoggingForOTE(e.Integration.GetAgentName(), oteName, e.lp)

	// Print to standard out.
	fmt.Println(e.text)
	// Print to the OTE logs and Cloud Logging.
	log.Logger.Info(e.text)
	// Log the action to usage metrics for usage reporting.
	usagemetrics.Action(actions.ExampleEchoFinished)
	return nil
}
