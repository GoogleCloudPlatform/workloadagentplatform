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

package statushelper

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"text/tabwriter"

	arpb "cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/testing/protocmp"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/commandlineexecutor"
	spb "github.com/GoogleCloudPlatform/workloadagentplatform/sharedprotos/status"
)

type mockArtifactRegistryClient struct {
	Versions []*arpb.Version
}

type mockVersionIterator struct {
	Versions []*arpb.Version
}

func (m *mockArtifactRegistryClient) ListVersions(ctx context.Context, req *arpb.ListVersionsRequest, opts ...gax.CallOption) VersionIterator {
	return &mockVersionIterator{Versions: m.Versions}
}

func (m *mockVersionIterator) Next() (*arpb.Version, error) {
	if len(m.Versions) == 0 {
		return nil, iterator.Done
	}
	version := m.Versions[0]
	m.Versions = m.Versions[1:]
	return version, nil
}

func fakeArtifactRegistryClient(versions []string) ARClientInterface {
	var arVersions []*arpb.Version
	for _, version := range versions {
		arVersions = append(arVersions, &arpb.Version{Name: version})
	}
	return &mockArtifactRegistryClient{Versions: arVersions}
}

type fakeExecutor struct {
	commandlineexecutor.Execute
	commandlineexecutor.Exists

	// fakeCommandRes maps commands to their results.
	fakeCommandRes map[string]commandlineexecutor.Result
}

func (e *fakeExecutor) ExecuteCommand(ctx context.Context, params commandlineexecutor.Params) commandlineexecutor.Result {
	for cmd, res := range e.fakeCommandRes {
		if cmd == params.Executable || strings.Contains(params.ArgsToSplit, cmd) {
			return res
		}
	}
	return commandlineexecutor.Result{Error: fmt.Errorf("cannot run command")}
}

func (e *fakeExecutor) CommandExists(cmd string) bool {
	if _, ok := e.fakeCommandRes[cmd]; ok {
		return true
	}
	return false
}

func TestLatestVersionArtifactRegistry(t *testing.T) {
	tests := []struct {
		name       string
		arClient   ARClientInterface
		wantLatest string
		wantErr    error
	}{
		{
			name:       "Success",
			arClient:   fakeArtifactRegistryClient([]string{"foo/0:1.1", "foo/0:3.5-671008012"}),
			wantLatest: "3.5-671008012",
		},
		{
			name:       "EmptyVersions",
			arClient:   fakeArtifactRegistryClient([]string{}),
			wantLatest: "",
			wantErr:    cmpopts.AnyError,
		},
		{
			name:       "UnsortedVersions",
			arClient:   fakeArtifactRegistryClient([]string{"foo/0:3.10-1", "foo/0:3.10-2", "foo/0:1.1-9", "foo/0:3.2-3"}),
			wantLatest: "3.10-2",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotLatest, gotErr := LatestVersionArtifactRegistry(context.Background(), test.arClient, "project-name", "us-central1", "repo", "foo")
			if !cmp.Equal(gotErr, test.wantErr, cmpopts.EquateErrors()) {
				t.Errorf("LatestVersionArtifactRegistry(%v) returned err: %v, wantErr: %v", test.arClient, gotErr, test.wantErr)
			}
			if diff := cmp.Diff(test.wantLatest, gotLatest); diff != "" {
				t.Errorf("LatestVersionArtifactRegistry(%v) returned unexpected diff (-want +got):\n%s", test.arClient, diff)
			}
		})
	}
}

func TestKernelVersion(t *testing.T) {
	tests := []struct {
		name    string
		osType  string
		exec    *fakeExecutor
		want    *spb.KernelVersion
		wantErr bool
	}{
		{
			name:   "LinuxCommandError",
			osType: "linux",
			exec: &fakeExecutor{
				fakeCommandRes: map[string]commandlineexecutor.Result{
					"uname": commandlineexecutor.Result{
						Error: fmt.Errorf("could not run command"),
					},
				},
			},
			wantErr: true,
		},
		{
			name:   "LinuxInvalidCommandOutput",
			osType: "linux",
			exec: &fakeExecutor{
				fakeCommandRes: map[string]commandlineexecutor.Result{
					"uname": commandlineexecutor.Result{
						StdOut: "1.2.3.4",
					},
				},
			},
			want: &spb.KernelVersion{
				RawString: "1.2.3.4",
			},
			wantErr: false,
		},
		{
			name:   "LinuxUnexpectedKernelVersion",
			osType: "linux",
			exec: &fakeExecutor{
				fakeCommandRes: map[string]commandlineexecutor.Result{
					"uname": commandlineexecutor.Result{
						StdOut: "Major.Minor.Build.Patch-102.17.1.el8.x86_64",
					},
				},
			},
			want: &spb.KernelVersion{
				RawString: "Major.Minor.Build.Patch-102.17.1.el8.x86_64",
				DistroKernel: &spb.KernelVersion_Version{
					Major:     102,
					Minor:     17,
					Build:     1,
					Patch:     0,
					Remainder: "el8.x86_64",
				},
			},
			wantErr: false,
		},
		{
			name:   "LinuxUnexpectedDistroVersion",
			osType: "linux",
			exec: &fakeExecutor{
				fakeCommandRes: map[string]commandlineexecutor.Result{
					"uname": commandlineexecutor.Result{
						StdOut: "5.15.102-Major.Minor.Build.Patch",
					},
				},
			},
			want: &spb.KernelVersion{
				RawString: "5.15.102-Major.Minor.Build.Patch",
				OsKernel: &spb.KernelVersion_Version{
					Major: 5,
					Minor: 15,
					Build: 102,
				},
			},
			wantErr: false,
		},
		{
			name:   "LinuxSuccessSLES",
			osType: "linux",
			exec: &fakeExecutor{
				fakeCommandRes: map[string]commandlineexecutor.Result{
					"uname": commandlineexecutor.Result{
						StdOut: "5.14.21-150500.55.73-default",
					},
				},
			},
			want: &spb.KernelVersion{
				RawString: "5.14.21-150500.55.73-default",
				OsKernel: &spb.KernelVersion_Version{
					Major: 5,
					Minor: 14,
					Build: 21,
				},
				DistroKernel: &spb.KernelVersion_Version{
					Major:     150500,
					Minor:     55,
					Build:     73,
					Remainder: "default",
				},
			},
			wantErr: false,
		},
		{
			name:   "LinuxSuccessRHEL",
			osType: "linux",
			exec: &fakeExecutor{
				fakeCommandRes: map[string]commandlineexecutor.Result{
					"uname": commandlineexecutor.Result{
						StdOut: "4.18.1.2-372.143.3.4.el8_6.x86_64",
					},
				},
			},
			want: &spb.KernelVersion{
				RawString: "4.18.1.2-372.143.3.4.el8_6.x86_64",
				OsKernel: &spb.KernelVersion_Version{
					Major: 4,
					Minor: 18,
					Build: 1,
					Patch: 2,
				},
				DistroKernel: &spb.KernelVersion_Version{
					Major:     372,
					Minor:     143,
					Build:     3,
					Patch:     4,
					Remainder: "el8_6.x86_64",
				},
			},
			wantErr: false,
		},
		{
			name:   "Windows",
			osType: "windows",
			exec: &fakeExecutor{
				fakeCommandRes: map[string]commandlineexecutor.Result{
					"uname": commandlineexecutor.Result{
						StdOut: "5.15.102-102.17.1.el8.x86_64",
					},
				},
			},
			wantErr: true,
		},
		{
			name:   "UnknownOS",
			osType: "unknown",
			exec: &fakeExecutor{
				fakeCommandRes: map[string]commandlineexecutor.Result{
					"uname": commandlineexecutor.Result{
						StdOut: "5.15.102-102.17.1.el8.x86_64",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := KernelVersion(context.Background(), test.osType, test.exec.ExecuteCommand)
			gotErr := err != nil
			if gotErr != test.wantErr {
				t.Errorf("KernelVersion() returned err: %v, wantErr: %v", err, test.wantErr)
			}
			if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("KernelVersion() returned unexpected diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckAgentEnabledAndRunning(t *testing.T) {
	tests := []struct {
		name        string
		agentName   string
		osType      string
		fakeCommand string
		fakeRes     commandlineexecutor.Result
		wantEnabled bool
		wantRunning bool
		wantErr     error
	}{
		{
			name:        "LinuxSuccess",
			agentName:   "foo",
			osType:      "linux",
			fakeCommand: "is-enabled",
			fakeRes: commandlineexecutor.Result{
				StdOut:   "enabled",
				ExitCode: 0,
			},
			wantEnabled: true,
			wantRunning: true,
		},
		{
			name:        "LinuxFailure",
			agentName:   "foo",
			osType:      "linux",
			fakeCommand: "is-enabled",
			fakeRes: commandlineexecutor.Result{
				ExitCode: 1,
				StdErr:   "could not refresh repositories",
				Error:    fmt.Errorf("could not refresh repositories"),
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name:        "WindowsSuccess",
			agentName:   "foo",
			osType:      "windows",
			fakeCommand: "Get-Service",
			fakeRes: commandlineexecutor.Result{
				StdOut:   "Running",
				ExitCode: 0,
			},
			wantEnabled: true,
			wantRunning: true,
		},
		{
			name:        "WindowsFailure",
			agentName:   "foo",
			osType:      "windows",
			fakeCommand: "Get-Service",
			fakeRes: commandlineexecutor.Result{
				ExitCode: 1,
				StdErr:   "could not get service status",
				Error:    fmt.Errorf("could not get service status"),
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name:      "UnsupportedOS",
			agentName: "foo",
			osType:    "unsupported",
			wantErr:   cmpopts.AnyError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exec := &fakeExecutor{fakeCommandRes: map[string]commandlineexecutor.Result{test.fakeCommand: test.fakeRes}}
			gotEnabled, gotRunning, gotErr := CheckAgentEnabledAndRunning(context.Background(), test.agentName, test.osType, exec.ExecuteCommand)
			if !cmp.Equal(gotErr, test.wantErr, cmpopts.EquateErrors()) {
				t.Errorf("CheckAgentEnabledAndRunning(%s, %s) returned err: %v, wantErr: %v", test.agentName, test.osType, gotErr, test.wantErr)
			}
			if diff := cmp.Diff(test.wantEnabled, gotEnabled); diff != "" {
				t.Errorf("CheckAgentEnabledAndRunning(%s, %s) returned unexpected enabled status diff (-want +got):\n%s", test.agentName, test.osType, diff)
			}
			if diff := cmp.Diff(test.wantRunning, gotRunning); diff != "" {
				t.Errorf("CheckAgentEnabledAndRunning(%s, %s) returned unexpected running status diff (-want +got):\n%s", test.agentName, test.osType, diff)
			}
		})
	}
}

func TestAgentEnabledAndRunningLinux(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		fakeCommand map[string]commandlineexecutor.Result
		wantEnabled bool
		wantRunning bool
		wantErr     error
	}{
		{
			name:        "ServiceEnabledAndRunning",
			serviceName: "foo",
			fakeCommand: map[string]commandlineexecutor.Result{
				"is-enabled": commandlineexecutor.Result{
					StdOut:   "enabled",
					ExitCode: 0,
				},
				"is-active": commandlineexecutor.Result{
					StdOut:   "active",
					ExitCode: 0,
				},
			},
			wantEnabled: true,
			wantRunning: true,
		},
		{
			name:        "ServiceEnabledButNotRunning",
			serviceName: "foo",
			fakeCommand: map[string]commandlineexecutor.Result{
				"is-enabled": commandlineexecutor.Result{
					StdOut:   "enabled",
					ExitCode: 0,
				},
				"is-active": commandlineexecutor.Result{
					StdOut:   "inactive",
					ExitCode: 1,
				},
			},
			wantEnabled: true,
			wantRunning: false,
		},
		{
			name:        "ServiceNotEnabledButRunning",
			serviceName: "foo",
			fakeCommand: map[string]commandlineexecutor.Result{
				"is-enabled": commandlineexecutor.Result{
					StdOut:   "disabled",
					ExitCode: 0,
				},
				"is-active": commandlineexecutor.Result{
					StdOut:   "active",
					ExitCode: 0,
				},
			},
			wantEnabled: false,
			wantRunning: true,
		},
		{
			name:        "ServiceNotEnabledAndNotRunning",
			serviceName: "foo",
			fakeCommand: map[string]commandlineexecutor.Result{
				"is-enabled": commandlineexecutor.Result{
					StdOut:   "disabled",
					ExitCode: 0,
				},
				"is-active": commandlineexecutor.Result{
					StdOut:   "inactive",
					ExitCode: 1,
				},
			},
			wantEnabled: false,
			wantRunning: false,
		},
		{
			name:        "ServiceNotEnabledAndNotRunningDifferentOutput",
			serviceName: "foo",
			fakeCommand: map[string]commandlineexecutor.Result{
				"is-enabled": commandlineexecutor.Result{
					StdOut:   "not enabled",
					ExitCode: 1,
				},
				"is-active": commandlineexecutor.Result{
					StdOut:   "inactive",
					ExitCode: 1,
				},
			},
			wantEnabled: false,
			wantRunning: false,
		},
		{
			name:        "ErrorCheckingEnabledStatus",
			serviceName: "foo",
			fakeCommand: map[string]commandlineexecutor.Result{
				"is-enabled": commandlineexecutor.Result{
					StdErr: "error checking enabled status",
					Error:  fmt.Errorf("error checking enabled status"),
				},
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name:        "ErrorCheckingRunningStatus",
			serviceName: "foo",
			fakeCommand: map[string]commandlineexecutor.Result{
				"is-enabled": commandlineexecutor.Result{
					StdOut:   "enabled",
					ExitCode: 0,
				},
				"is-active": commandlineexecutor.Result{
					StdErr: "error checking running status",
					Error:  fmt.Errorf("error checking running status"),
				},
			},
			wantErr: cmpopts.AnyError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exec := &fakeExecutor{
				fakeCommandRes: test.fakeCommand,
			}
			gotEnabled, gotRunning, err := agentEnabledAndRunningLinux(context.Background(), test.serviceName, exec.ExecuteCommand)
			if !cmp.Equal(err, test.wantErr, cmpopts.EquateErrors()) {
				t.Errorf("agentEnabledAndRunningLinux(%s) returned err: %v, wantErr: %v", test.serviceName, err, test.wantErr)
			}
			if diff := cmp.Diff(test.wantEnabled, gotEnabled); diff != "" {
				t.Errorf("agentEnabledAndRunningLinux(%s) returned unexpected enabled status diff (-want +got):\n%s", test.serviceName, diff)
			}
			if diff := cmp.Diff(test.wantRunning, gotRunning); diff != "" {
				t.Errorf("agentEnabledAndRunningLinux(%s) returned unexpected running status diff (-want +got):\n%s", test.serviceName, diff)
			}
		})
	}
}

func TestAgentEnabledAndRunningWindows(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		fakeCommand map[string]commandlineexecutor.Result
		wantEnabled bool
		wantRunning bool
		wantErr     error
	}{
		{
			name:        "ServiceRunning",
			serviceName: "foo",
			fakeCommand: map[string]commandlineexecutor.Result{
				"Get-Service": commandlineexecutor.Result{
					StdOut:   " Running ",
					ExitCode: 0,
				},
			},
			wantEnabled: true,
			wantRunning: true,
		},
		{
			name:        "ServiceStopped",
			serviceName: "foo",
			fakeCommand: map[string]commandlineexecutor.Result{
				"Get-Service": commandlineexecutor.Result{
					StdOut:   " Stopped ",
					ExitCode: 0,
				},
			},
			wantEnabled: false,
			wantRunning: false,
		},
		{
			name:        "ServiceNotStoppedOrRunning",
			serviceName: "foo",
			fakeCommand: map[string]commandlineexecutor.Result{
				"Get-Service": commandlineexecutor.Result{
					StdOut:   "",
					ExitCode: 0,
				},
			},
			wantEnabled: false,
			wantRunning: false,
			wantErr:     cmpopts.AnyError,
		},
		{
			name:        "ErrorCheckingEnabledStatus",
			serviceName: "foo",
			fakeCommand: map[string]commandlineexecutor.Result{
				"Get-Service": commandlineexecutor.Result{
					StdErr: "error checking running status",
					Error:  fmt.Errorf("error checking running status"),
				},
			},
			wantEnabled: false,
			wantRunning: false,
			wantErr:     cmpopts.AnyError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exec := &fakeExecutor{
				fakeCommandRes: test.fakeCommand,
			}
			gotEnabled, gotRunning, err := agentEnabledAndRunningWindows(context.Background(), test.serviceName, exec.ExecuteCommand)
			if !cmp.Equal(err, test.wantErr, cmpopts.EquateErrors()) {
				t.Errorf("agentEnabledAndRunningWindows(%s) returned err: %v, wantErr: %v", test.serviceName, err, test.wantErr)
			}
			if diff := cmp.Diff(test.wantEnabled, gotEnabled); diff != "" {
				t.Errorf("agentEnabledAndRunningWindows(%s) returned unexpected enabled status diff (-want +got):\n%s", test.serviceName, diff)
			}
			if diff := cmp.Diff(test.wantRunning, gotRunning); diff != "" {
				t.Errorf("agentEnabledAndRunningWindows(%s) returned unexpected running status diff (-want +got):\n%s", test.serviceName, diff)
			}
		})
	}
}

func TestPrintStatus(t *testing.T) {
	tests := []struct {
		name    string
		status  *spb.AgentStatus
		compact bool
		want    string
	}{
		{
			name:   "emptyStatus",
			status: &spb.AgentStatus{},
			want: `--------------------------------------------------------------------------------
|                                    Status                                    |
--------------------------------------------------------------------------------
Agent Status:
    Installed Version: 
    Available Version: 
    Systemd Service Enabled: Error: could not determine status
    Systemd Service Running: Error: could not determine status
    Cloud API Full Scopes:   Error: could not determine status
    Configuration File: 
    Configuration Valid: Error: could not determine status
        


`,
		},
		{
			name: "fullStatusAllSuccess",
			status: &spb.AgentStatus{
				AgentName:                       "Agent for SAP",
				InstalledVersion:                "3.6",
				AvailableVersion:                "3.6",
				SystemdServiceEnabled:           spb.State_SUCCESS_STATE,
				SystemdServiceRunning:           spb.State_SUCCESS_STATE,
				ConfigurationFilePath:           "/etc/google-cloud-sap-agent/configuration.json",
				ConfigurationValid:              spb.State_SUCCESS_STATE,
				ConfigurationErrorMessage:       "error: proto: (line 6:44): invalid value for bool field value: 2",
				CloudApiAccessFullScopesGranted: spb.State_SUCCESS_STATE,
				Services: []*spb.ServiceStatus{
					{
						Name:            "Process Metrics",
						State:           spb.State_SUCCESS_STATE,
						FullyFunctional: spb.State_SUCCESS_STATE,
						ErrorMessage:    "Cannot write to Cloud Monitoring, check IAM permissions",
						IamPermissions: []*spb.IAMPermission{
							{
								Name:    "example.compute.viewer",
								Granted: spb.State_SUCCESS_STATE,
							},
							{
								Name:    "example.monitoring.viewer",
								Granted: spb.State_SUCCESS_STATE,
							},
						},
						ConfigValues: []*spb.ConfigValue{
							{
								Name:      "collect_process_metrics",
								Value:     "True",
								IsDefault: true,
							},
							{
								Name:      "process_metrics_frequency",
								Value:     "5",
								IsDefault: true,
							},
						},
					},
					{
						Name:            "Host Metrics",
						State:           spb.State_SUCCESS_STATE,
						FullyFunctional: spb.State_SUCCESS_STATE,
						ErrorMessage:    "Cannot write to Cloud Monitoring, check IAM permissions",
						ConfigValues: []*spb.ConfigValue{
							{
								Name:      "Hello",
								Value:     "World",
								IsDefault: true,
							},
						},
					},
					{
						Name:            "Backint",
						State:           spb.State_FAILURE_STATE,
						FullyFunctional: spb.State_FAILURE_STATE,
						ErrorMessage:    "Cannot write to Cloud Monitoring, check IAM permissions",
						ConfigValues: []*spb.ConfigValue{
							{
								Name:      "fake_config",
								Value:     "5",
								IsDefault: true,
							},
						},
					},
				},
				References: []*spb.Reference{
					{
						Name: "IAM Permissions",
						Url:  "https://cloud.google.com/solutions/sap/docs/agent-for-sap/latest/planning#required_iam_roles",
					},
					{
						Name: "What's New",
						Url:  "https://cloud.google.com/solutions/sap/docs/agent-for-sap/whats-new",
					},
				},
			},
			want: `--------------------------------------------------------------------------------
|                             Agent for SAP Status                             |
--------------------------------------------------------------------------------
Agent Status:
    Installed Version: 3.6
    Available Version: 3.6
    Systemd Service Enabled: True
    Systemd Service Running: True
    Cloud API Full Scopes:   True
    Configuration File: /etc/google-cloud-sap-agent/configuration.json
    Configuration Valid: True
--------------------------------------------------------------------------------
Process Metrics: Enabled
    Status: Fully Functional
    IAM Permissions: All granted
    Configuration:
        collect_process_metrics:   True (default)
        process_metrics_frequency: 5    (default)
--------------------------------------------------------------------------------
Host Metrics: Enabled
    Status: Fully Functional
    Configuration:
        Hello: World (default)
--------------------------------------------------------------------------------
Backint: Disabled
--------------------------------------------------------------------------------
References:
IAM Permissions: https://cloud.google.com/solutions/sap/docs/agent-for-sap/latest/planning#required_iam_roles
What's New: https://cloud.google.com/solutions/sap/docs/agent-for-sap/whats-new


`,
		},
		{
			name:    "fullStatusAllSuccessQuiet",
			compact: true,
			status: &spb.AgentStatus{
				AgentName:                       "Agent for SAP",
				InstalledVersion:                "3.6",
				AvailableVersion:                "3.6",
				SystemdServiceEnabled:           spb.State_SUCCESS_STATE,
				SystemdServiceRunning:           spb.State_SUCCESS_STATE,
				ConfigurationFilePath:           "/etc/google-cloud-sap-agent/configuration.json",
				ConfigurationValid:              spb.State_SUCCESS_STATE,
				ConfigurationErrorMessage:       "error: proto: (line 6:44): invalid value for bool field value: 2",
				CloudApiAccessFullScopesGranted: spb.State_SUCCESS_STATE,
				Services: []*spb.ServiceStatus{
					{
						Name:            "Process Metrics",
						State:           spb.State_SUCCESS_STATE,
						FullyFunctional: spb.State_SUCCESS_STATE,
						ErrorMessage:    "Cannot write to Cloud Monitoring, check IAM permissions",
						IamPermissions: []*spb.IAMPermission{
							{
								Name:    "example.compute.viewer",
								Granted: spb.State_SUCCESS_STATE,
							},
							{
								Name:    "example.monitoring.viewer",
								Granted: spb.State_SUCCESS_STATE,
							},
						},
						ConfigValues: []*spb.ConfigValue{
							{
								Name:      "collect_process_metrics",
								Value:     "True",
								IsDefault: true,
							},
							{
								Name:      "process_metrics_frequency",
								Value:     "5",
								IsDefault: true,
							},
						},
					},
					{
						Name:            "Host Metrics",
						State:           spb.State_SUCCESS_STATE,
						FullyFunctional: spb.State_SUCCESS_STATE,
						ErrorMessage:    "Cannot write to Cloud Monitoring, check IAM permissions",
						ConfigValues: []*spb.ConfigValue{
							{
								Name:      "Hello",
								Value:     "World",
								IsDefault: true,
							},
						},
					},
					{
						Name:            "Backint",
						State:           spb.State_FAILURE_STATE,
						FullyFunctional: spb.State_FAILURE_STATE,
						ErrorMessage:    "Cannot write to Cloud Monitoring, check IAM permissions",
						ConfigValues: []*spb.ConfigValue{
							{
								Name:      "fake_config",
								Value:     "5",
								IsDefault: true,
							},
						},
					},
				},
				References: []*spb.Reference{
					{
						Name: "IAM Permissions",
						Url:  "https://cloud.google.com/solutions/sap/docs/agent-for-sap/latest/planning#required_iam_roles",
					},
					{
						Name: "What's New",
						Url:  "https://cloud.google.com/solutions/sap/docs/agent-for-sap/whats-new",
					},
				},
			},
			want: `--------------------------------------------------------------------------------
|                             Agent for SAP Status                             |
--------------------------------------------------------------------------------
Agent Status:
    Installed Version: 3.6
    Available Version: 3.6
    Systemd Service Enabled: True
    Systemd Service Running: True
    Cloud API Full Scopes:   True
    Configuration File: /etc/google-cloud-sap-agent/configuration.json
    Configuration Valid: True
--------------------------------------------------------------------------------
Process Metrics: Enabled
    Status: Fully Functional
    IAM Permissions: All granted
--------------------------------------------------------------------------------
Host Metrics: Enabled
    Status: Fully Functional
--------------------------------------------------------------------------------
Backint: Disabled
--------------------------------------------------------------------------------
References:
IAM Permissions: https://cloud.google.com/solutions/sap/docs/agent-for-sap/latest/planning#required_iam_roles
What's New: https://cloud.google.com/solutions/sap/docs/agent-for-sap/whats-new


`,
		},
		{
			name: "fullStatusWithFailures",
			status: &spb.AgentStatus{
				AgentName:                       "Agent for SAP",
				InstalledVersion:                "3.5",
				AvailableVersion:                "3.6",
				SystemdServiceEnabled:           spb.State_FAILURE_STATE,
				SystemdServiceRunning:           spb.State_FAILURE_STATE,
				ConfigurationFilePath:           "/etc/google-cloud-sap-agent/configuration.json",
				ConfigurationValid:              spb.State_FAILURE_STATE,
				ConfigurationErrorMessage:       "error: proto: (line 6:44): invalid value for bool field value: 2",
				CloudApiAccessFullScopesGranted: spb.State_FAILURE_STATE,
				Services: []*spb.ServiceStatus{
					{
						Name:            "Process Metrics",
						State:           spb.State_SUCCESS_STATE,
						FullyFunctional: spb.State_FAILURE_STATE,
						ErrorMessage:    "Cannot write to Cloud Monitoring, check IAM permissions",
						IamPermissions: []*spb.IAMPermission{
							{Name: "example.compute.viewer", Granted: spb.State_SUCCESS_STATE},
							{Name: "example.monitoring.viewer", Granted: spb.State_ERROR_STATE},
							{Name: "example.failed", Granted: spb.State_FAILURE_STATE},
							{Name: "example.failed", Granted: spb.State_FAILURE_STATE},
							{Name: "example.failed", Granted: spb.State_FAILURE_STATE},
							{Name: "example.failed", Granted: spb.State_FAILURE_STATE},
							{Name: "example.failed", Granted: spb.State_ERROR_STATE},
						},
						ConfigValues: []*spb.ConfigValue{
							{
								Name:      "collect_process_metrics",
								Value:     "True",
								IsDefault: false,
							},
							{
								Name:      "process_metrics_frequency",
								Value:     "",
								IsDefault: true,
							},
						},
					},
					{
						Name:            "Host Metrics",
						State:           spb.State_SUCCESS_STATE,
						FullyFunctional: spb.State_UNSPECIFIED_STATE,
						ConfigValues: []*spb.ConfigValue{
							{
								Name:      "Hello",
								Value:     "World",
								IsDefault: true,
							},
						},
					},
					{
						Name:                    "Backint",
						State:                   spb.State_UNSPECIFIED_STATE,
						UnspecifiedStateMessage: "Backint parameters file not specified / Disabled",
						FullyFunctional:         spb.State_FAILURE_STATE,
						ErrorMessage:            "Cannot write to Cloud Monitoring, check IAM permissions",
						ConfigValues: []*spb.ConfigValue{
							{
								Name:      "fake_config",
								Value:     "5",
								IsDefault: true,
							},
						},
					},
				},
				References: []*spb.Reference{
					{
						Name: "IAM Permissions",
						Url:  "https://cloud.google.com/solutions/sap/docs/agent-for-sap/latest/planning#required_iam_roles",
					},
					{
						Name: "What's New",
						Url:  "https://cloud.google.com/solutions/sap/docs/agent-for-sap/whats-new",
					},
				},
			},
			want: `--------------------------------------------------------------------------------
|                             Agent for SAP Status                             |
--------------------------------------------------------------------------------
Agent Status:
    Installed Version: 3.5
    Available Version: 3.6
    Systemd Service Enabled: False
    Systemd Service Running: False
    Cloud API Full Scopes:   False
    Configuration File: /etc/google-cloud-sap-agent/configuration.json
    Configuration Valid: False
        error: proto: (line 6:44): invalid value for bool field value: 2
--------------------------------------------------------------------------------
Process Metrics: Enabled
    Status: Error: Cannot write to Cloud Monitoring, check IAM permissions
    IAM Permissions: 6 not granted (output limited to 5)
        example.failed:            False
        example.failed:            False
        example.failed:            False
        example.failed:            False
        example.monitoring.viewer: Error: could not determine status
    Configuration:
        collect_process_metrics:   True (configuration file)
        process_metrics_frequency: nil  (default)
--------------------------------------------------------------------------------
Host Metrics: Enabled
    Status: Error: could not determine status
    Configuration:
        Hello: World (default)
--------------------------------------------------------------------------------
Backint: Backint parameters file not specified / Disabled
--------------------------------------------------------------------------------
References:
IAM Permissions: https://cloud.google.com/solutions/sap/docs/agent-for-sap/latest/planning#required_iam_roles
What's New: https://cloud.google.com/solutions/sap/docs/agent-for-sap/whats-new


`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Monkey patch the tab writer to check the output.
			defer func(oldWriter *tabwriter.Writer) {
				tabWriter = oldWriter
			}(tabWriter)
			r, w, _ := os.Pipe()
			tabWriter = tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
			PrintStatus(context.Background(), tc.status, tc.compact)

			w.Close()
			var buf bytes.Buffer
			io.Copy(&buf, r)

			// NOTE: The //third_party/golang/fatihcolor/color package does some
			// helpful tricks to detect it's not able to support colors in the go
			// test environment so the text here has no special characters
			got := buf.String()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("PrintStatus(%v) had unexpected diff (-want +got):\n%s", tc.status, diff)
			}
		})
	}
}
