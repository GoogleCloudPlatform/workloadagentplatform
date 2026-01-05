/*
Copyright 2025 Google LLC

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

package guestactions

import (
	"context"
	"errors"
	"fmt"
	"testing"

	anypb "google.golang.org/protobuf/types/known/anypb"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/testing/protocmp"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/commandlineexecutor"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/gce/metadataserver"

	gpb "github.com/GoogleCloudPlatform/workloadagentplatform/sharedprotos/guestactions"
)

var testHandlers = map[string]GuestActionHandler{
	"version": func(ctx context.Context, command *gpb.Command, cloudProperties *metadataserver.CloudProperties) *gpb.CommandResult {
		return &gpb.CommandResult{
			Command:  command,
			Stdout:   fmt.Sprintf("Google Cloud Agent for SAP version test response"),
			Stderr:   "",
			ExitCode: 0,
		}
	},
}

func TestAnyResponse(t *testing.T) {
	tests := []struct {
		name    string
		gar     *gpb.GuestActionResponse
		want    *anypb.Any
		wantErr bool
	}{
		{
			name: "Standard",
			gar: &gpb.GuestActionResponse{
				CommandResults: []*gpb.CommandResult{
					{
						Command: &gpb.Command{
							CommandType: &gpb.Command_AgentCommand{
								AgentCommand: &gpb.AgentCommand{Command: "version"},
							},
						},
						Stdout:   fmt.Sprintf("Google Cloud Agent for SAP version test response"),
						Stderr:   "",
						ExitCode: 0,
					},
				},
				Error: &gpb.GuestActionError{ErrorMessage: ""},
			},
			want: &anypb.Any{
				TypeUrl: "type.googleapis.com/workloadagentplatform.sharedprotos.guestactions.GuestActionResponse",
			},
		},
		{
			name: "Nil",
			gar:  nil,
			want: &anypb.Any{
				TypeUrl: "type.googleapis.com/workloadagentplatform.sharedprotos.guestactions.GuestActionResponse",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			got := anyResponse(ctx, test.gar)
			if diff := cmp.Diff(test.want.GetTypeUrl(), got.GetTypeUrl()); diff != "" {
				t.Errorf("anyResponse(%q) returned diff (-want +got):\n%s", test.name, diff)
			}
		})
	}
}

func TestParseRequest(t *testing.T) {
	tests := []struct {
		name    string
		msg     *anypb.Any
		want    *gpb.GuestActionRequest
		wantErr bool
	}{
		{
			name: "Success",
			msg: &anypb.Any{
				TypeUrl: "type.googleapis.com/workloadagentplatform.sharedprotos.guestactions.GuestActionRequest",
				Value:   []byte{},
			},
			want:    &gpb.GuestActionRequest{},
			wantErr: false,
		},
		{
			name: "InvalidMsgValue",
			msg: &anypb.Any{
				TypeUrl: "type.googleapis.com/workloadagentplatform.sharedprotos.guestactions.GuestActionRequest",
				Value:   []byte("invalid"),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "InvalidMsgType",
			msg: &anypb.Any{
				TypeUrl: "type.googleapis.com/workloadagentplatform.invalid.proto.GuestActionRequest",
				Value:   []byte{},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := parseRequest(ctx, test.msg)
			if test.wantErr && err == nil {
				t.Errorf("parseRequest(%q) returned nil error, want error", test.name)
			}
			if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("parseRequest(%q) returned diff (-want +got):\n%s", test.name, diff)
			}
		})
	}
}

func TestHandleShellCommand(t *testing.T) {
	tests := []struct {
		name    string
		command *gpb.Command
		want    *gpb.CommandResult
		execute commandlineexecutor.Execute
	}{
		{
			name: "ShellCommandError",
			command: &gpb.Command{
				CommandType: &gpb.Command_ShellCommand{
					ShellCommand: &gpb.ShellCommand{Command: "eecho", Args: "Hello World!"},
				},
			},
			want: &gpb.CommandResult{
				Command: &gpb.Command{
					CommandType: &gpb.Command_ShellCommand{
						ShellCommand: &gpb.ShellCommand{Command: "eecho", Args: "Hello World!"},
					},
				},
				Stdout:   "",
				Stderr:   "Command executable: \"eecho\" not found.",
				ExitCode: 1,
			},
			execute: commandlineexecutor.ExecuteCommand,
		},
		{
			name: "ShellCommandErrorNonZeroStatus",
			command: &gpb.Command{
				CommandType: &gpb.Command_ShellCommand{
					ShellCommand: &gpb.ShellCommand{Command: "eecho", Args: "Hello World!"},
				},
			},
			want: &gpb.CommandResult{
				Command: &gpb.Command{
					CommandType: &gpb.Command_ShellCommand{
						ShellCommand: &gpb.ShellCommand{Command: "eecho", Args: "Hello World!"},
					},
				},
				Stdout:   "",
				Stderr:   "",
				ExitCode: 3,
			},
			execute: func(ctx context.Context, params commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					ExitCode: 3,
				}
			},
		},
		{
			name: "ShellCommandErrorZeroStatus",
			command: &gpb.Command{
				CommandType: &gpb.Command_ShellCommand{
					ShellCommand: &gpb.ShellCommand{Command: "eecho", Args: "Hello World!"},
				},
			},
			want: &gpb.CommandResult{
				Command: &gpb.Command{
					CommandType: &gpb.Command_ShellCommand{
						ShellCommand: &gpb.ShellCommand{Command: "eecho", Args: "Hello World!"},
					},
				},
				Stdout:   "",
				Stderr:   "",
				ExitCode: 1,
			},
			execute: func(ctx context.Context, params commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					Error:    errors.New("command executable: \"eecho\" not found"),
					ExitCode: 0,
				}
			},
		},
		{
			name: "ShellCommandStderrZeroStatus",
			command: &gpb.Command{
				CommandType: &gpb.Command_ShellCommand{
					ShellCommand: &gpb.ShellCommand{Command: "eecho", Args: "Hello World!"},
				},
			},
			want: &gpb.CommandResult{
				Command: &gpb.Command{
					CommandType: &gpb.Command_ShellCommand{
						ShellCommand: &gpb.ShellCommand{Command: "eecho", Args: "Hello World!"},
					},
				},
				Stdout:   "",
				Stderr:   "Command executable: \"eecho\" not found.",
				ExitCode: 1,
			},
			execute: func(ctx context.Context, params commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdErr:   "Command executable: \"eecho\" not found.",
					ExitCode: 0,
				}
			},
		},
		{
			name: "ShellCommandSuccess",
			command: &gpb.Command{
				CommandType: &gpb.Command_ShellCommand{
					ShellCommand: &gpb.ShellCommand{Command: "echo", Args: "Hello World!"},
				},
			},
			want: &gpb.CommandResult{
				Command: &gpb.Command{
					CommandType: &gpb.Command_ShellCommand{
						ShellCommand: &gpb.ShellCommand{Command: "echo", Args: "Hello World!"},
					},
				},
				Stdout:   "Hello World!\n",
				Stderr:   "",
				ExitCode: 0,
			},
			execute: commandlineexecutor.ExecuteCommand,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			got := handleShellCommand(ctx, test.command, test.execute)
			if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("handleShellCommand(%v) returned diff (-want +got):\n%s", test.command, diff)
			}
		})
	}
}

func TestProcessCommands(t *testing.T) {
	tests := []struct {
		name    string
		message *gpb.GuestActionRequest
		want    []*gpb.CommandResult
		wantErr bool
	}{
		{
			name:    "NoCommands",
			message: &gpb.GuestActionRequest{},
			want:    nil,
			wantErr: false,
		},
		{
			name: "UnknownCommandType",
			message: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{{}},
			},
			want: []*gpb.CommandResult{
				&gpb.CommandResult{
					Command:  nil,
					Stdout:   "received unknown command: ",
					Stderr:   "received unknown command: ",
					ExitCode: 1,
				},
			},
			wantErr: true,
		},
		{
			name: "AgentCommand",
			message: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "version"},
						},
					},
				},
			},
			want: []*gpb.CommandResult{
				{
					Command: &gpb.Command{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "version"},
						},
					},
					Stdout:   fmt.Sprintf("Google Cloud Agent for SAP version test response"),
					Stderr:   "",
					ExitCode: 0,
				},
			},
			wantErr: false,
		},
		{
			name: "UnknownAgentCommand",
			message: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "unknown_command"},
						},
					},
				},
			},
			want: []*gpb.CommandResult{
				{
					Command: &gpb.Command{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "unknown_command"},
						},
					},
					Stdout: fmt.Sprintf("received unknown agent command: %s", prototext.Format(&gpb.Command{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "unknown_command"},
						},
					})),
					Stderr: fmt.Sprintf("received unknown agent command: %s", prototext.Format(&gpb.Command{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "unknown_command"},
						},
					})),
					ExitCode: 1,
				},
			},
			wantErr: true,
		},
		{
			name: "ShellCommandError",
			message: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_ShellCommand{
							ShellCommand: &gpb.ShellCommand{Command: "eecho", Args: "Hello World!"},
						},
					},
				},
			},
			want: []*gpb.CommandResult{
				{
					Command: &gpb.Command{
						CommandType: &gpb.Command_ShellCommand{
							ShellCommand: &gpb.ShellCommand{Command: "eecho", Args: "Hello World!"},
						},
					},
					Stdout:   "",
					Stderr:   "Command executable: \"eecho\" not found.",
					ExitCode: 1,
				},
			},
			wantErr: true,
		},
		{
			name: "ShellCommandSuccess",
			message: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_ShellCommand{
							ShellCommand: &gpb.ShellCommand{Command: "echo", Args: "Hello World!"},
						},
					},
				},
			},
			want: []*gpb.CommandResult{
				{
					Command: &gpb.Command{
						CommandType: &gpb.Command_ShellCommand{
							ShellCommand: &gpb.ShellCommand{Command: "echo", Args: "Hello World!"},
						},
					},
					Stdout:   "Hello World!\n",
					Stderr:   "",
					ExitCode: 0,
				},
			},
			wantErr: false,
		},
	}

	ga := &GuestActions{
		options: Options{
			Handlers: testHandlers,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := ga.processCommands(ctx, test.message, nil)
			if test.wantErr {
				if err == nil {
					t.Errorf("processCommands(%q) returned nil error, want error", test.name)
				}
			} else if err != nil {
				t.Errorf("processCommands(%q) returned error: %v, want nil error", test.name, err)
			}

			if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("processCommands(%q) returned diff (-want +got):\n%s", test.name, diff)
			}
		})
	}
}

func TestIsLRORequest(t *testing.T) {
	dummyHandler := func(ctx context.Context, command *gpb.Command, cloudProperties *metadataserver.CloudProperties) *gpb.CommandResult {
		return &gpb.CommandResult{
			Command:  command,
			Stdout:   "dummy handler called",
			ExitCode: 0,
		}
	}
	ga := GuestActions{
		options: Options{
			Handlers: map[string]GuestActionHandler{
				"synccommand": dummyHandler,
			},
			LROHandlers: map[string]GuestActionHandler{
				"lrocommand": dummyHandler,
			},
		},
	}

	tests := []struct {
		name    string
		request *gpb.GuestActionRequest
		want    bool
	}{
		{
			name:    "NoCommands",
			request: &gpb.GuestActionRequest{},
			want:    false,
		},
		{
			name: "LROCommand",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "lrocommand"},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "SyncCommand",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "synccommand"},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "ShellCommand",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_ShellCommand{
							ShellCommand: &gpb.ShellCommand{Command: "echo"},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "UnknownAgentCommand",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "unknown"},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "CommandWithNoType",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{},
				},
			},
			want: false,
		},
		{
			name: "MixedCommandsWithLRO",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "synccommand"},
						},
					},
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "lrocommand"},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "MixedCommandsWithoutLRO",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "synccommand"},
						},
					},
					{
						CommandType: &gpb.Command_ShellCommand{
							ShellCommand: &gpb.ShellCommand{Command: "echo"},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "ShellCommandThenLRO",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_ShellCommand{
							ShellCommand: &gpb.ShellCommand{Command: "echo"},
						},
					},
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "lrocommand"},
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ga.isLRORequest(tc.request)
			if got != tc.want {
				t.Errorf("isLRORequest() got %v, want %v", got, tc.want)
			}
		})
	}
}
