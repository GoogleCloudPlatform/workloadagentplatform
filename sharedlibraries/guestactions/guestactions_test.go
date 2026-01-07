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
	"sort"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/agentcommunication_client"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/testing/protocmp"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/commandlineexecutor"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/communication"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/gce/metadataserver"

	anypb "google.golang.org/protobuf/types/known/anypb"
	acpb "github.com/GoogleCloudPlatform/agentcommunication_client/gapic/agentcommunicationpb"
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

func TestLockerAcquireAndRelease(t *testing.T) {
	type stepAction int
	const (
		actionLock stepAction = iota
		actionRelease
		sleep
	)
	type step struct {
		action      stepAction
		acquireReq  map[string]time.Duration // for acquire action
		releaseReq  []string                 // for release action
		wantAcquire bool                     // for acquire action
		wantBusyKey string                   // for acquire action
		duration    time.Duration            // for sleep action
	}
	tests := []struct {
		name  string
		steps []step
	}{
		{
			name: "single key",
			steps: []step{
				{action: actionLock, acquireReq: map[string]time.Duration{"a": defaultLockTimeout}, wantAcquire: true, wantBusyKey: ""},
				{action: actionLock, acquireReq: map[string]time.Duration{"a": defaultLockTimeout}, wantAcquire: false, wantBusyKey: "a"},
				{action: actionRelease, releaseReq: []string{"a"}},
				{action: actionLock, acquireReq: map[string]time.Duration{"a": defaultLockTimeout}, wantAcquire: true, wantBusyKey: ""},
				{action: actionRelease, releaseReq: []string{"a"}},
			},
		},
		{
			name: "multiple keys",
			steps: []step{
				{action: actionLock, acquireReq: map[string]time.Duration{"a": defaultLockTimeout, "b": defaultLockTimeout}, wantAcquire: true, wantBusyKey: ""},
				{action: actionLock, acquireReq: map[string]time.Duration{"a": defaultLockTimeout}, wantAcquire: false, wantBusyKey: "a"},
				{action: actionLock, acquireReq: map[string]time.Duration{"b": defaultLockTimeout}, wantAcquire: false, wantBusyKey: "b"},
				{action: actionLock, acquireReq: map[string]time.Duration{"c": defaultLockTimeout}, wantAcquire: true, wantBusyKey: ""},
				{action: actionRelease, releaseReq: []string{"b"}},
				{action: actionLock, acquireReq: map[string]time.Duration{"a": defaultLockTimeout}, wantAcquire: false, wantBusyKey: "a"},
				{action: actionLock, acquireReq: map[string]time.Duration{"b": defaultLockTimeout}, wantAcquire: true, wantBusyKey: ""},
				{action: actionRelease, releaseReq: []string{"a", "b", "c"}},
			},
		},
		{
			name: "intersecting keys",
			steps: []step{
				{action: actionLock, acquireReq: map[string]time.Duration{"a": defaultLockTimeout}, wantAcquire: true, wantBusyKey: ""},
				{action: actionLock, acquireReq: map[string]time.Duration{"a": defaultLockTimeout, "b": defaultLockTimeout}, wantAcquire: false, wantBusyKey: "a"},
				{action: actionRelease, releaseReq: []string{"a"}},
				{action: actionLock, acquireReq: map[string]time.Duration{"a": defaultLockTimeout, "b": defaultLockTimeout}, wantAcquire: true, wantBusyKey: ""},
				{action: actionRelease, releaseReq: []string{"a", "b"}},
			},
		},
		{
			name: "no keys",
			steps: []step{
				{action: actionLock, acquireReq: map[string]time.Duration{}, wantAcquire: true, wantBusyKey: ""},
				{action: actionRelease, releaseReq: []string{}},
				{action: actionLock, acquireReq: map[string]time.Duration{}, wantAcquire: true, wantBusyKey: ""},
			},
		},
		{
			name: "empty string key",
			steps: []step{
				{action: actionLock, acquireReq: map[string]time.Duration{"": defaultLockTimeout}, wantAcquire: true, wantBusyKey: ""},
				{action: actionLock, acquireReq: map[string]time.Duration{"": defaultLockTimeout}, wantAcquire: false, wantBusyKey: ""},
				{action: actionRelease, releaseReq: []string{""}},
				{action: actionLock, acquireReq: map[string]time.Duration{"": defaultLockTimeout}, wantAcquire: true, wantBusyKey: ""},
				{action: actionRelease, releaseReq: []string{""}},
			},
		},
		{
			name: "lock expires",
			steps: []step{
				{action: actionLock, acquireReq: map[string]time.Duration{"a": 1 * time.Millisecond}, wantAcquire: true, wantBusyKey: ""},
				{action: actionLock, acquireReq: map[string]time.Duration{"a": 1 * time.Millisecond}, wantAcquire: false, wantBusyKey: "a"},
				{action: sleep, duration: 2 * time.Millisecond},
				{action: actionLock, acquireReq: map[string]time.Duration{"a": 1 * time.Millisecond}, wantAcquire: true, wantBusyKey: ""},
				{action: actionRelease, releaseReq: []string{"a"}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := newLocker()
			for i, step := range tc.steps {
				switch step.action {
				case actionLock:
					if key, ok := l.acquire(step.acquireReq); ok != step.wantAcquire || key != step.wantBusyKey {
						t.Errorf("Step %d: Acquire(%v) = (%q, %v), want: (%q, %v)", i, step.acquireReq, key, ok, step.wantBusyKey, step.wantAcquire)
					}
				case actionRelease:
					l.release(step.releaseReq)
				case sleep:
					time.Sleep(step.duration)
				}
			}
		})
	}
}

func TestAcquireLocksForRequest(t *testing.T) {
	tests := []struct {
		name               string
		request            *gpb.GuestActionRequest
		concurrencyKeyFunc func(*gpb.Command) (string, time.Duration, bool)
		wantKeys           []string
		wantBusyKey        string
		wantOk             bool
		existingLocks      map[string]time.Duration
	}{
		{
			name:    "NoCommands",
			request: &gpb.GuestActionRequest{},
			concurrencyKeyFunc: func(*gpb.Command) (string, time.Duration, bool) {
				return "key", defaultLockTimeout, true
			},
			wantKeys: []string{},
			wantOk:   true,
		},
		{
			name: "NoConcurrencyKeyFunc",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "cmd1"},
						},
					},
				},
			},
			concurrencyKeyFunc: nil,
			wantKeys:           []string{},
			wantOk:             true,
		},
		{
			name: "ConcurrencyKeyFuncReturnsFalse",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "cmd1"},
						},
					},
				},
			},
			concurrencyKeyFunc: func(*gpb.Command) (string, time.Duration, bool) {
				return "", 0, false
			},
			wantKeys: []string{},
			wantOk:   true,
		},
		{
			name: "SingleCommandLockSuccess",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "cmd1"},
						},
					},
				},
			},
			concurrencyKeyFunc: func(*gpb.Command) (string, time.Duration, bool) {
				return "key1", defaultLockTimeout, true
			},
			wantKeys: []string{"key1"},
			wantOk:   true,
		},
		{
			name: "SingleCommandLockFailed",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "cmd1"},
						},
					},
				},
			},
			concurrencyKeyFunc: func(*gpb.Command) (string, time.Duration, bool) {
				return "key1", defaultLockTimeout, true
			},
			existingLocks: map[string]time.Duration{"key1": defaultLockTimeout},
			wantBusyKey:   "key1",
			wantOk:        false,
		},
		{
			name: "MultipleCommandsSameKeyLockSuccess",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "cmd1"},
						},
					},
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "cmd2"},
						},
					},
				},
			},
			concurrencyKeyFunc: func(c *gpb.Command) (string, time.Duration, bool) {
				if c.GetAgentCommand().GetCommand() == "cmd1" {
					return "key1", 1 * time.Minute, true
				}
				return "key1", 2 * time.Minute, true
			},
			wantKeys: []string{"key1"},
			wantOk:   true,
		},
		{
			name: "MultipleCommandsDifferentKeysLockSuccess",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "cmd1"},
						},
					},
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "cmd2"},
						},
					},
				},
			},
			concurrencyKeyFunc: func(c *gpb.Command) (string, time.Duration, bool) {
				if c.GetAgentCommand().GetCommand() == "cmd1" {
					return "key1", 0, true
				}
				return "key2", 0, true
			},
			wantKeys: []string{"key1", "key2"},
			wantOk:   true,
		},
		{
			name: "MultipleCommandsOneKeyLocked",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "cmd1"},
						},
					},
					{
						CommandType: &gpb.Command_AgentCommand{
							AgentCommand: &gpb.AgentCommand{Command: "cmd2"},
						},
					},
				},
			},
			concurrencyKeyFunc: func(c *gpb.Command) (string, time.Duration, bool) {
				if c.GetAgentCommand().GetCommand() == "cmd1" {
					return "key1", 0, true
				}
				return "key2", 0, true
			},
			existingLocks: map[string]time.Duration{"key2": defaultLockTimeout},
			wantBusyKey:   "key2",
			wantOk:        false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			g := &GuestActions{
				options: Options{
					CommandConcurrencyKey: tc.concurrencyKeyFunc,
				},
				locker: newLocker(),
			}
			if tc.existingLocks != nil {
				g.locker.acquire(tc.existingLocks)
			}

			keys, busyKey, ok := g.acquireLocksForRequest(ctx, tc.request)

			if ok != tc.wantOk {
				t.Errorf("acquireLocksForRequest() ok = %v, want %v", ok, tc.wantOk)
			}
			if busyKey != tc.wantBusyKey {
				t.Errorf("acquireLocksForRequest() busyKey = %q, want %q", busyKey, tc.wantBusyKey)
			}

			// Sort keys for consistent comparison
			sort.Strings(keys)
			sort.Strings(tc.wantKeys)
			if diff := cmp.Diff(tc.wantKeys, keys); diff != "" {
				t.Errorf("acquireLocksForRequest() keys returned diff (-want +got):\n%s", diff)
			}
			if !tc.wantOk && len(g.locker.locks) != len(tc.existingLocks) {
				t.Errorf("acquireLocksForRequest() should not acquire new locks on failure, got %d locks, want %d", len(g.locker.locks), len(tc.existingLocks))
			}
		})
	}
}

func TestExecuteAndSendDone(t *testing.T) {
	tests := []struct {
		name         string
		request      *gpb.GuestActionRequest
		wantStatus   string
		wantExitCode int32
	}{
		{
			name: "Success",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_ShellCommand{
							ShellCommand: &gpb.ShellCommand{Command: "echo", Args: "Hello World!"},
						},
					},
				},
			},
			wantStatus:   statusSucceeded,
			wantExitCode: 0,
		},
		{
			name: "Failure",
			request: &gpb.GuestActionRequest{
				Commands: []*gpb.Command{
					{
						CommandType: &gpb.Command_ShellCommand{
							ShellCommand: &gpb.ShellCommand{Command: "eecho", Args: "Hello World!"},
						},
					},
				},
			},
			wantStatus:   statusFailed,
			wantExitCode: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			g := &GuestActions{
				options: Options{
					Handlers: testHandlers,
				},
				locker: newLocker(),
			}
			locks := map[string]time.Duration{"test-key": 1 * time.Minute}
			if _, ok := g.locker.acquire(locks); !ok {
				t.Fatalf("Acquire(%v) failed", locks)
			}

			var gotMsg *acpb.MessageBody
			origSendMessage := communication.SendMessage
			defer func() { communication.SendMessage = origSendMessage }()
			communication.SendMessage = func(c *client.Connection, msg *acpb.MessageBody) error {
				gotMsg = msg
				return nil
			}

			g.executeAndSendDone(ctx, "op1", tc.request, nil, nil, []string{"test-key"})

			if gotMsg == nil {
				t.Fatalf("executeAndSendDone() did not send message")
			}
			if gotMsg.Labels["state"] != tc.wantStatus {
				t.Errorf("executeAndSendDone() sent status %q, want %q", gotMsg.Labels["state"], tc.wantStatus)
			}
			resp := &gpb.GuestActionResponse{}
			if err := gotMsg.GetBody().UnmarshalTo(resp); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}
			if len(resp.GetCommandResults()) != 1 {
				t.Fatalf("executeAndSendDone() returned %d command results, want 1", len(resp.GetCommandResults()))
			}
			if resp.GetCommandResults()[0].GetExitCode() != tc.wantExitCode {
				t.Errorf("executeAndSendDone() returned exit code %d, want %d", resp.GetCommandResults()[0].GetExitCode(), tc.wantExitCode)
			}

			// Verify lock is released
			if key, ok := g.locker.acquire(locks); !ok {
				t.Errorf("Acquire(%v) failed after executeAndSendDone(), want success, locked by %s", locks, key)
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
