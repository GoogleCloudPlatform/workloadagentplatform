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

// Package main serves as the Main entry point for the Google Cloud Example Agent.
package main

import (
	"context"
	"os"
	"runtime"
	"time"

	"flag"
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
	"github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/onetime"
	"github.com/GoogleCloudPlatform/workloadagentplatform/integration/example/cmd/persistentflags"
	"github.com/GoogleCloudPlatform/workloadagentplatform/integration/example/onetime/echo"
	"github.com/GoogleCloudPlatform/workloadagentplatform/integration/example/onetime/logusage"
	"github.com/GoogleCloudPlatform/workloadagentplatform/integration/example/service"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/gce/metadataserver"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"

	cpb "github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/protos"
)

// AgentBuildChange is the change number that the agent was built at
var AgentBuildChange = `0`

var rootCmd *cobra.Command

var agentIntegration = &cpb.Integration{
	IntegrationName: `example`,
	AgentName:       `google-cloud-example-agent`,
	AgentBinaryName: `google_cloud_example_agent`,
	AgentLongName:   `Google Cloud Example Agent`,
	AgentVersion:    `1.0.` + AgentBuildChange,
}

func registerSubCommands(ctx context.Context, lp log.Parameters, cp *cpb.CloudProperties) {
	rootCmd = &cobra.Command{
		Use: agentIntegration.AgentBinaryName,
	}
	rootCmd.AddCommand(onetime.NewVersionCommand(agentIntegration))
	rootCmd.AddCommand(echo.NewEcho(ctx, lp, cp, agentIntegration))
	rootCmd.AddCommand(logusage.NewLogUsageCommand(ctx, lp, cp, agentIntegration))

	daemon := service.NewDaemon(lp, cp, agentIntegration)
	service.PopulateDaemonFlagValues(daemon, rootCmd.Flags())
	daemonCommand := service.NewDaemonSubcommand(ctx, daemon)
	// When running on windows, the daemon is started using the winservice subcommand.
	// Having both the daemon command and the winservice command will cause an error when the
	// winservice tries to start the daemon, cobra will start the parent which is the winservice
	// causing a loop.
	if lp.OSType != "windows" {
		rootCmd.AddCommand(daemonCommand)
	}

	// Disabling plugin capabilities until it is ready for release.
	// plugin := service.NewPlugin(daemon)
	// service.PopulatePluginFlagValues(plugin, rootCmd.Flags())
	// pluginCommand := service.NewPluginSubcommand(plugin)
	// rootCmd.AddCommand(pluginCommand)

	// Add any additional windows or linux specific subcommands.
	rootCmd.AddCommand(additionalSubcommands(ctx, agentIntegration, daemonCommand)...)

	rootCmd.SetArgs(flag.Args())
	// Persistent flags are set at the root command level and available to all subcommands.
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() != "startdaemon" {
			persistentflags.Register(agentIntegration.GetAgentName(), lp.OSType, cmd)
		}
	}
}

func main() {
	ctx := context.Background()
	lp := log.Parameters{
		OSType:     runtime.GOOS,
		Level:      zapcore.InfoLevel,
		LogToCloud: true,
	}

	cloudProps := &cpb.CloudProperties{}
	if cp := metadataserver.FetchCloudProperties(); cp != nil {
		cloudProps = &cpb.CloudProperties{
			ProjectId:        cp.ProjectID,
			InstanceId:       cp.InstanceID,
			Zone:             cp.Zone,
			InstanceName:     cp.InstanceName,
			Image:            cp.Image,
			NumericProjectId: cp.NumericProjectID,
			MachineType:      cp.MachineType,
		}
	}
	lp.CloudLoggingClient = log.CloudLoggingClient(ctx, cloudProps.GetProjectId())
	registerSubCommands(ctx, lp, cloudProps)

	rc := 0
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		log.Logger.Error(err)
		rc = 1
	}

	// Defer cloud log flushing to ensure execution on any exit from main.
	defer func() {
		if lp.CloudLoggingClient != nil {
			flushTimer := time.AfterFunc(5*time.Second, func() {
				log.Logger.Error("Cloud logging client failed to flush logs within the 5-second deadline, exiting.")
				os.Exit(rc)
			})
			log.FlushCloudLog()
			lp.CloudLoggingClient.Close()
			flushTimer.Stop()
		}
	}()

	os.Exit(rc)
}
