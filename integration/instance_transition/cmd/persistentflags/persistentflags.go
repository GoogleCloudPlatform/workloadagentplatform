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

// Package persistentflags is used to configure and parse persistent flags across all subcommands.
package persistentflags

import (
	"github.com/spf13/cobra"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"
)

// Persistent flags (defined at the root command level)
var (
	logFile, logLevel string
	logToCloud        bool
)

// Register registers the persistent flags for the root command.
func Register(agentName string, osType string, rootCmd *cobra.Command) {
	fp := log.DefaultOTEPath(agentName, osType, "")
	rootCmd.PersistentFlags().StringVarP(&logFile, "log-file", "f", fp, "Set the file path for logging")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "Set the logging level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVarP(&logToCloud, "log-to-cloud", "c", true, "Enable logging to the cloud")
}

// SetValues sets the persistent flags for the subcommand.
func SetValues(agentName string, lp *log.Parameters, cmd *cobra.Command) {
	flags := cmd.Flags()
	logToCloud, _ := flags.GetBool("log-to-cloud")
	logLevel, _ := flags.GetString("log-level")
	logFile, _ := flags.GetString("log-file")

	lp.LogToCloud = logToCloud
	lp.Level = log.StringLevelToZapcore(logLevel)
	lp.LogFileName = log.OTEFilePath(agentName, cmd.Name(), lp.OSType, "")
	if logFile != log.DefaultOTEPath(agentName, lp.OSType, "") {
		lp.LogFileName = logFile
	}
}
