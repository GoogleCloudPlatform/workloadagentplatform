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

// Package service contains all of the code for the example agents services.
package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/GoogleCloudPlatform/workloadagentplatform/integration/common"
	"github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/usagemetrics"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/recovery"

	cpb "github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/protos"
	epb "github.com/GoogleCloudPlatform/workloadagentplatform/integration/example/protos"
)

// Daemon has args for daemon subcommand.
type Daemon struct {
	Integration    *cpb.Integration
	configFilePath string
	lp             log.Parameters
	config         *epb.Configuration
	cloudProps     *cpb.CloudProperties
	services       []Service
}

type (
	// Service defines the common interface for integration services.
	// Start method is used to start the integration services.
	Service interface {
		Start(ctx context.Context, a any)
		String() string
		ErrorCode() int
		ExpectedMinDuration() time.Duration
	}
)

const (
	// LinuxConfigPath is the default path to agent configuration file on linux.
	LinuxConfigPath = `/etc/google-cloud-example-agent/configuration.json`
	// WindowsConfigPath is the default path to agent configuration file on windows.
	WindowsConfigPath = `C:\Program Files\Google\google-cloud-example-agent\conf\configuration.json`
)

// NewDaemon creates a new Daemon.
func NewDaemon(lp log.Parameters, cloudProps *cpb.CloudProperties, integration *cpb.Integration) *Daemon {
	return &Daemon{
		lp:          lp,
		cloudProps:  cloudProps,
		Integration: integration,
	}
}

// PopulateDaemonFlagValues uses the provided flags to set the daemon's primitive values.
func PopulateDaemonFlagValues(daemon *Daemon, fs *pflag.FlagSet) {
	fs.StringVar(&daemon.configFilePath, "config", "", "configuration path for daemon mode")
	fs.StringVar(&daemon.configFilePath, "c", "", "configuration path for daemon mode")
}

// NewDaemonSubcommand creates a new Command using the provided daemon for work.
func NewDaemonSubcommand(ctx context.Context, daemon *Daemon) *cobra.Command {
	daemonCmd := &cobra.Command{
		Use:   "startdaemon",
		Short: "Start daemon mode of the agent",
		Long:  "startdaemon [--config <path-to-config-file>]",
		RunE: func(cmd *cobra.Command, args []string) error {
			return daemon.Execute(cmd.Context())
		},
	}
	daemonCmd.Flags().StringVar(&daemon.configFilePath, "config", "", "configuration path for daemon mode")
	daemonCmd.Flags().StringVar(&daemon.configFilePath, "c", "", "configuration path for daemon mode")
	daemonCmd.SetContext(ctx)
	return daemonCmd
}

// Execute runs the daemon command.
func (d *Daemon) Execute(ctx context.Context) error {
	// Setup daemon logging
	d.lp.CloudLogName = d.Integration.GetAgentName()
	d.lp.LogFileName = fmt.Sprintf("/var/log/%s.log", d.Integration.GetAgentName())
	if d.lp.OSType == "windows" {
		d.lp.LogFileName = fmt.Sprintf(`%s\Google\%s\logs\%s.log`, log.CreateWindowsLogBasePath(), d.Integration.GetAgentName(), d.Integration.GetAgentName())
	}
	log.SetupLogging(d.lp)

	// Run the daemon handler that will start any services for the integration.
	ctx, cancel := context.WithCancel(ctx)
	d.daemonHandler(ctx, cancel)
	return nil
}

func (d *Daemon) applyConfigurationDefaults() {
	// Set the default values for the configuration as needed.
	if d.config.GetFastServiceLoopSeconds() == 0 {
		d.config.FastServiceLoopSeconds = 5
	}
	if d.config.GetSlowServiceLoopSeconds() == 0 {
		d.config.SlowServiceLoopSeconds = 30
	}
}

func (d *Daemon) daemonHandler(ctx context.Context, cancel context.CancelFunc) error {
	if d.configFilePath == "" {
		d.configFilePath = LinuxConfigPath
	}
	if d.lp.OSType == "windows" {
		d.configFilePath = WindowsConfigPath
	}
	log.Logger.Infof("Reading configuration from %s", d.configFilePath)
	c, err := common.Read(d.configFilePath, os.ReadFile, &epb.Configuration{})
	if err != nil {
		return err
	}
	log.Logger.Info("Configuration read successfully")
	d.config = c.(*epb.Configuration)
	d.applyConfigurationDefaults()
	d.lp.LogToCloud = d.config.GetLogToCloud().GetValue()
	d.lp.Level = common.LogLevelToZapcore(d.config.GetLogLevel().Number())
	if d.config.GetCloudProperties().GetProjectId() != "" {
		d.lp.CloudLoggingClient = log.CloudLoggingClient(ctx, d.config.GetCloudProperties().GetProjectId())
	}
	if d.lp.CloudLoggingClient != nil {
		defer d.lp.CloudLoggingClient.Close()
	}
	// setup logging with the configuration read from the config file.
	log.SetupLogging(d.lp)
	log.Logger.Infow("Agent version currently running", "version", d.Integration.GetAgentVersion())
	log.Logger.Infow("Cloud Properties from metadata server",
		"projectid", d.cloudProps.GetProjectId(),
		"projectnumber", d.cloudProps.GetNumericProjectId(),
		"instanceid", d.cloudProps.GetInstanceId(),
		"zone", d.cloudProps.GetZone(),
		"instancename", d.cloudProps.GetInstanceName(),
		"image", d.cloudProps.GetImage())

	d.configureUsageMetricsForDaemon(d.cloudProps)
	usagemetrics.Started()
	log.Logger.Info("Daemon started")

	shutdownch := make(chan os.Signal, 1)
	signal.Notify(shutdownch, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	// Start the services for the integration.
	d.services = []Service{
		&Fast{config: d.config},
		&Slow{config: d.config},
	}
	for _, service := range d.services {
		log.Logger.Infof("Daemon starting service: %s", service.String())
		recoverableStart := &recovery.RecoverableRoutine{
			Routine:             service.Start,
			ErrorCode:           service.ErrorCode(),
			ExpectedMinDuration: service.ExpectedMinDuration(),
			UsageLogger:         *usagemetrics.UsageLogger,
		}
		recoverableStart.StartRoutine(ctx)
	}
	// Log a RUNNING usage metric once a day.
	go usagemetrics.LogRunningDaily()
	// Wait for the shutdown signal.
	d.waitForShutdown(ctx, shutdownch, cancel)
	return nil
}

func (d *Daemon) configureUsageMetricsForDaemon(cp *cpb.CloudProperties) {
	usagemetrics.SetProperties(d.Integration.GetAgentName(), d.Integration.GetAgentVersion(), true, cp)
}

// waitForShutdown observes a channel for a shutdown signal, then proceeds to shut down the Agent.
func (d *Daemon) waitForShutdown(ctx context.Context, ch <-chan os.Signal, cancel context.CancelFunc) {
	// wait for the shutdown signal
	<-ch
	log.Logger.Info("Shutdown signal observed, the agent will begin shutting down")
	cancel()
	usagemetrics.Stopped()
	time.Sleep(3 * time.Second)
	log.Logger.Info("Shutting down...")
}
