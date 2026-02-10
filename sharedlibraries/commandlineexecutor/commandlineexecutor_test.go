/*
Copyright 2022 Google LLC

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

package commandlineexecutor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func setDefaults() {
	exists = CommandExists
	exitCode = commandExitCode
	run = nil
	exeForPlatform = nil
}

type fakeUserResolver struct {
	uid    uint32
	gid    uint32
	groups []uint32
	err    error
}

func (m *fakeUserResolver) lookupIDs(username string) (uint32, uint32, []uint32, error) {
	return m.uid, m.gid, m.groups, m.err
}

func TestExecuteCommandWithArgsToSplit(t *testing.T) {
	input := []struct {
		name    string
		cmd     string
		args    string
		wantOut string
		wantErr string
	}{
		{
			name:    "echo",
			cmd:     "echo",
			args:    "hello, world",
			wantOut: "hello, world\n",
			wantErr: "",
		},
		{
			name:    "bashMd5sum",
			cmd:     "bash",
			args:    "-c 'echo $0 | md5sum' 'test hashing functions'",
			wantOut: "664ff1cd20fea05ce7bbbf36ef0be984  -\n",
			wantErr: "",
		},
		{
			name:    "bashSha1sum",
			cmd:     "bash",
			args:    "-c 'echo test | sha1sum'",
			wantOut: "4e1243bd22c66e76c2ba9eddc1f91394e57f9f83  -\n",
			wantErr: "",
		},
	}
	for _, test := range input {
		t.Run(test.name, func(t *testing.T) {
			setDefaults()
			result := ExecuteCommand(context.Background(), Params{
				Executable:  test.cmd,
				ArgsToSplit: test.args,
			})
			if result.Error != nil {
				t.Fatalf("ExecuteCommand with argstosplit returned unexpected error: %v", result.Error)
			}
			if diff := cmp.Diff(test.wantOut, result.StdOut); diff != "" {
				t.Fatalf("ExecuteCommand with argstosplit returned unexpected diff (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(test.wantErr, result.StdErr); diff != "" {
				t.Fatalf("ExecuteCommand with argstosplit returned unexpected diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExecuteCommandWithArgs(t *testing.T) {
	input := []struct {
		name    string
		cmd     string
		args    []string
		wantOut string
		wantErr string
	}{
		{
			name:    "echo",
			cmd:     "echo",
			args:    []string{"hello, world"},
			wantOut: "hello, world\n",
			wantErr: "",
		},
		{
			name:    "env",
			cmd:     "env",
			args:    []string{"--", "echo", "test | sha1sum"},
			wantOut: "test | sha1sum\n",
			wantErr: "",
		},
		{
			name:    "bashSha1sum",
			cmd:     "bash",
			args:    []string{"-c", "echo $0$1$2 | sha1sum", "section1,", "section2,", "section3"},
			wantOut: "28b0c645455ccefe28044c7e24eaeb8e80aa40d4  -\n",
			wantErr: "",
		},
		{
			name:    "shMd5sum",
			cmd:     "sh",
			args:    []string{"-c", "echo $0 | md5sum", "test hashing functions with sh"},
			wantOut: "664a07d6f2be061a942820847888865d  -\n",
			wantErr: "",
		},
	}
	for _, test := range input {
		t.Run(test.name, func(t *testing.T) {
			setDefaults()
			result := ExecuteCommand(context.Background(), Params{
				Executable: test.cmd,
				Args:       test.args,
			})
			if result.Error != nil {
				t.Fatal(result.Error)
			}
			if diff := cmp.Diff(test.wantOut, result.StdOut); diff != "" {
				t.Fatalf("ExecuteCommand with args returned unexpected diff (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(test.wantErr, result.StdErr); diff != "" {
				t.Fatalf("ExecuteCommand with args returned unexpected diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCommandExists(t *testing.T) {
	input := []struct {
		name   string
		cmd    string
		exists bool
	}{
		{
			name:   "echoExists",
			cmd:    "echo",
			exists: true,
		},
		{
			name:   "lsExists",
			cmd:    "ls",
			exists: true,
		},
		{
			name:   "encryptDoesNotExist",
			cmd:    "encrypt",
			exists: false,
		},
	}
	for _, test := range input {
		t.Run(test.name, func(t *testing.T) {
			setDefaults()
			if got := CommandExists(test.cmd); got != test.exists {
				t.Fatalf("CommandExists returned unexpected result, got: %t want: %t", got, test.exists)
			}
		})
	}
}

func TestExecuteCommandAsUser(t *testing.T) {
	tests := []struct {
		name         string
		cmd          string
		fakeExists   Exists
		fakeRun      Run
		fakeExitCode ExitCode
		fakeSetupExe SetupExeForPlatform
		wantExitCode int64
		wantErr      error
	}{
		{
			name:         "ExistingCmd",
			cmd:          "ls",
			fakeExists:   func(string) bool { return true },
			fakeRun:      func() error { return nil },
			fakeSetupExe: func(exe *exec.Cmd, params Params) error { return nil },
			wantErr:      nil,
		},
		{
			name:         "NonExistingCmd",
			cmd:          "encrypt",
			fakeExists:   func(string) bool { return false },
			wantExitCode: 0,
			wantErr:      cmpopts.AnyError,
		},
		{
			name:         "ExitCode15",
			cmd:          "ls",
			fakeExists:   func(string) bool { return true },
			fakeRun:      func() error { return fmt.Errorf("some failure") },
			fakeExitCode: func(error) int { return 15 },
			fakeSetupExe: func(exe *exec.Cmd, params Params) error { return nil },
			wantExitCode: 15,
			wantErr:      cmpopts.AnyError,
		},
		{
			name:       "NoExitCodeDoNotPanic",
			cmd:        "echo",
			fakeExists: func(string) bool { return true },
			fakeRun: func() error {
				return fmt.Errorf("exit status no-num")
			},
			fakeExitCode: func(error) int { return 1 },
			fakeSetupExe: func(exe *exec.Cmd, params Params) error { return nil },
			wantErr:      cmpopts.AnyError,
			wantExitCode: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exists = test.fakeExists
			exitCode = test.fakeExitCode
			run = test.fakeRun
			exeForPlatform = test.fakeSetupExe
			result := ExecuteCommand(context.Background(), Params{
				Executable: test.cmd,
				User:       "test-user",
			})

			if !cmp.Equal(result.Error, test.wantErr, cmpopts.EquateErrors()) {
				t.Fatalf("ExecuteCommand with user got an error: %v, want: %v", result.Error, test.wantErr)
			}
			if test.wantExitCode != int64(result.ExitCode) {
				t.Fatalf("ExecuteCommand with user got an unexpected exit code: %d, want: %d", result.ExitCode, test.wantExitCode)
			}
		})
	}
}

func TestExecuteWithEnv(t *testing.T) {
	tests := []struct {
		name         string
		params       Params
		wantStdOut   string
		wantExitCode int
		wantErr      error
	}{
		{
			name: "ExistingCmd",
			params: Params{
				Executable:  "echo",
				ArgsToSplit: "test",
			},
			wantStdOut:   "test\n",
			wantExitCode: 0,
			wantErr:      nil,
		},
		{
			name: "NonExistingCmd",
			params: Params{
				Executable: "encrypt",
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name: "CommandFailure",
			params: Params{
				Executable:  "cat",
				ArgsToSplit: "nonexisting.txtjson",
			},
			wantExitCode: 1,
			wantErr:      cmpopts.AnyError,
		},
		{
			name: "InvalidUser",
			params: Params{
				Executable: "ls",
				User:       "invalidUser",
			},
			wantErr: cmpopts.AnyError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setDefaults()

			result := ExecuteCommand(context.Background(), test.params)

			if !cmp.Equal(result.Error, test.wantErr, cmpopts.EquateErrors()) {
				t.Errorf("ExecuteCommand with env got error: %v, want: %v", result.Error, test.wantErr)
			}
			if test.wantExitCode != result.ExitCode {
				t.Errorf("ExecuteCommand with env got exit code: %d, want: %d", result.ExitCode, test.wantExitCode)
			}
			if diff := cmp.Diff(test.wantStdOut, result.StdOut); diff != "" {
				t.Errorf("ExecuteCommand with env returned unexpected diff (-want +got):\n%s", diff)
			}
		})
	}
}

type mockUserResolver struct {
	uid    uint32
	gid    uint32
	groups []uint32
	err    error
}

func (m *mockUserResolver) lookupIDs(username string) (uint32, uint32, []uint32, error) {
	return m.uid, m.gid, m.groups, m.err
}

func TestSetupExeForPlatform(t *testing.T) {
	tests := []struct {
		name             string
		params           Params
		userResolver     userResolver
		executeCommand   *mockExecute
		wantEnv          []string
		wantCred         *syscall.Credential
		wantErr          bool
		wantLookupIDsErr bool
	}{
		{
			name:   "EnvOnly",
			params: Params{Env: []string{"VAR1=val1"}},
			wantEnv: []string{
				"VAR1=val1",
			},
		},
		{
			name:   "UserOnlyLookupSuccess",
			params: Params{User: "testuser"},
			userResolver: &mockUserResolver{
				uid: 1001, gid: 1002, groups: []uint32{1002, 2001},
			},
			wantCred: &syscall.Credential{Uid: 1001, Gid: 1002, Groups: []uint32{1002, 2001}},
		},
		{
			name:   "UserOnlyLookupFailsFetchSuccess",
			params: Params{User: "testuser"},
			userResolver: &mockUserResolver{
				err: errors.New("lookup failed"),
			},
			executeCommand: &mockExecute{
				uidResult:  Result{StdOut: "1001\n"},
				gidResult:  Result{StdOut: "1002\n"},
				gidsResult: Result{StdOut: "1002 2001\n"},
				t:          t,
			},
			wantCred: &syscall.Credential{Uid: 1001, Gid: 1002, Groups: []uint32{1002, 2001}},
		},
		{
			name:   "UserOnlyLookupFailsFetchFails",
			params: Params{User: "testuser"},
			userResolver: &mockUserResolver{
				err: errors.New("lookup failed"),
			},
			executeCommand: &mockExecute{
				uidResult: Result{Error: errors.New("fetch failed")},
				t:         t,
			},
			wantErr: true,
		},
		{
			name: "EnvAndUser",
			params: Params{
				Env:  []string{"VAR1=val1"},
				User: "testuser",
			},
			userResolver: &mockUserResolver{
				uid: 1001, gid: 1002, groups: []uint32{1002, 2001},
			},
			wantEnv:  []string{"VAR1=val1"},
			wantCred: &syscall.Credential{Uid: 1001, Gid: 1002, Groups: []uint32{1002, 2001}},
		},
		{
			name:   "NoEnvNoUser",
			params: Params{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exe := &exec.Cmd{}
			var execute Execute
			if tt.executeCommand != nil {
				execute = tt.executeCommand.Execute
			}
			err := setupExeForPlatform(context.Background(), exe, tt.params, execute, tt.userResolver)
			if (err != nil) != tt.wantErr {
				t.Errorf("setupExeForPlatform() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantEnv != nil {
				for _, want := range tt.wantEnv {
					found := false
					for _, got := range exe.Env {
						if got == want {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("setupExeForPlatform() missing env var = %v, want in %v", want, exe.Env)
					}
				}
			}

			if tt.wantCred != nil {
				if exe.SysProcAttr == nil || exe.SysProcAttr.Credential == nil {
					t.Errorf("setupExeForPlatform() SysProcAttr.Credential is nil, want %v", tt.wantCred)
				} else if diff := cmp.Diff(tt.wantCred, exe.SysProcAttr.Credential); diff != "" {
					t.Errorf("setupExeForPlatform() SysProcAttr.Credential mismatch (-want +got):\n%s", diff)
				}
			} else if exe.SysProcAttr != nil {
				t.Errorf("setupExeForPlatform() SysProcAttr is not nil, want nil")
			}
		})
	}
}

func TestSplitParams(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantOut []string
	}{
		{
			name:    "echo",
			args:    "echo hello, world",
			wantOut: []string{"echo", "hello,", "world"},
		},
		{
			name:    "bashMd5sum",
			args:    "-c 'echo $0 | md5sum' 'test hashing functions'",
			wantOut: []string{"-c", "echo $0 | md5sum", "test hashing functions"},
		},
		{
			name:    "tcpFiltering",
			args:    "-c 'lsof -nP -p $(pidof hdbnameserver) | grep LISTEN | grep -v 127.0.0.1 | grep -Eo `(([0-9]{1,3}\\.){1,3}[0-9]{1,3})|(\\*)\\:[0-9]{3,5}`'",
			wantOut: []string{"-c", "lsof -nP -p $(pidof hdbnameserver) | grep LISTEN | grep -v 127.0.0.1 | grep -Eo '(([0-9]{1,3}\\.){1,3}[0-9]{1,3})|(\\*)\\:[0-9]{3,5}'"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			splitArgs := splitParams(test.args)
			if diff := cmp.Diff(test.wantOut, splitArgs); diff != "" {
				t.Fatalf("splitParams returned unexpected diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExecuteCommandWithStdin(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		args    []string
		input   string
		wantOut string
		wantErr string
	}{
		{
			name:    "grep hello",
			cmd:     "grep",
			args:    []string{"hello"},
			input:   "hello world\nhello Go\nbye world\n",
			wantOut: "hello world\nhello Go\n",
			wantErr: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setDefaults()
			result := ExecuteCommand(context.Background(), Params{
				Executable: test.cmd,
				Args:       test.args,
				Stdin:      test.input,
			})
			if result.Error != nil {
				t.Fatal(result.Error)
			}
			if diff := cmp.Diff(test.wantOut, result.StdOut); diff != "" {
				t.Fatalf("ExecuteCommand returned unexpected diff (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(test.wantErr, result.StdErr); diff != "" {
				t.Fatalf("ExecuteCommand returned unexpected diff (-want +got):\n%s", diff)
			}
		})
	}
}
