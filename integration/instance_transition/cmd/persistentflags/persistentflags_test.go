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

package persistentflags

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap/zapcore"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"
)

func executeCommand(root *cobra.Command, args ...string) (c *cobra.Command, output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	c, err = root.ExecuteC()

	return c, buf.String(), err
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name           string
		os             string
		agent          string
		wantFileName   string
		wantLevel      zapcore.Level
		wantLogToCloud bool
	}{
		{
			name:           "Windows",
			os:             "windows",
			agent:          "test-agent",
			wantFileName:   `C:\Program Files\Google\test-agent\logs\test-agent-{COMMAND}.log`,
			wantLevel:      zapcore.InfoLevel,
			wantLogToCloud: true,
		},
		{
			name:           "Linux",
			os:             "linux",
			agent:          "test-agent",
			wantFileName:   `/var/log/test-agent-{COMMAND}.log`,
			wantLevel:      zapcore.InfoLevel,
			wantLogToCloud: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := &cobra.Command{
				Use:  "root",
				Args: cobra.ArbitraryArgs,
				Run:  func(_ *cobra.Command, args []string) {},
			}
			Register(test.agent, test.os, cmd)
			cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
				if f.Name == "log-file" {
					if f.Value.String() != test.wantFileName {
						t.Errorf("Register(%s, %s) log-file flag is incorrect, got: %s, want: %s", test.agent, test.os, f.Value.String(), test.wantFileName)
					}
				}
				if f.Name == "log-level" {
					if f.Value.String() != test.wantLevel.String() {
						t.Errorf("Register(%s, %s) log-level flag is incorrect, got: %s, want: %s", test.agent, test.os, f.Value.String(), test.wantLevel.String())
					}
				}
				if f.Name == "log-to-cloud" {
					wantLog := strconv.FormatBool(test.wantLogToCloud)
					if f.Value.String() != wantLog {
						t.Errorf("Register(%s, %s) log-to-cloud flag is incorrect, got: %s, want: %s", test.agent, test.os, f.Value.String(), wantLog)
					}
				}
			})
		})
	}
}

func TestSetValues(t *testing.T) {
	tests := []struct {
		name           string
		os             string
		agent          string
		args           []string
		wantFileName   string
		wantLevel      zapcore.Level
		wantLogToCloud bool
	}{
		{
			name:           "Windows",
			agent:          "test-agent",
			os:             "windows",
			wantFileName:   `C:\Program Files\Google\test-agent\logs\test-agent-echo.log`,
			wantLevel:      zapcore.InfoLevel,
			wantLogToCloud: true,
		},
		{
			name:           "Linux",
			agent:          "test-agent",
			os:             "linux",
			wantFileName:   `/var/log/test-agent-echo.log`,
			wantLevel:      zapcore.InfoLevel,
			wantLogToCloud: true,
		},
		{
			name:           "SetArgs",
			agent:          "test-agent",
			os:             "linux",
			args:           []string{"--log-level=debug", "--log-to-cloud=false", "--log-file=/test.log"},
			wantFileName:   `/test.log`,
			wantLevel:      zapcore.DebugLevel,
			wantLogToCloud: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := &cobra.Command{
				Use:  "echo",
				Args: cobra.ArbitraryArgs,
				Run:  func(_ *cobra.Command, args []string) {},
			}
			Register(test.agent, test.os, cmd)
			c, output, err := executeCommand(cmd, test.args...)
			if output != "" {
				t.Errorf("Unexpected output: %v", output)
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			lp := &log.Parameters{OSType: test.os}
			SetValues(test.agent, lp, c)
			if lp.LogFileName != test.wantFileName {
				t.Errorf("SetValues(%s) log-file is incorrect, got: %s, want: %s", test.agent, lp.LogFileName, test.wantFileName)
			}
			if lp.Level != test.wantLevel {
				t.Errorf("SetValues(%s) log-level is incorrect, got: %s, want: %s", test.agent, lp.Level.String(), test.wantLevel.String())
			}
			if lp.LogToCloud != test.wantLogToCloud {
				t.Errorf("SetValues(%s) log-to-cloud is incorrect, got: %v, want: %v", test.agent, lp.LogToCloud, test.wantLogToCloud)
			}
		})
	}
}
