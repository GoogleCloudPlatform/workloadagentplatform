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
	"os/user"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLookupIDs(t *testing.T) {
	tests := []struct {
		name         string
		username     string
		fakeLookup   func(string) (*user.User, error)
		fakeGroupIds func(*user.User) ([]string, error)
		wantUID      uint32
		wantGID      uint32
		wantGroups   []uint32
		wantErr      bool
	}{
		{
			name:     "Success",
			username: "testuser",
			fakeLookup: func(u string) (*user.User, error) {
				return &user.User{Uid: "1001", Gid: "1001", Username: "testuser"}, nil
			},
			fakeGroupIds: func(u *user.User) ([]string, error) {
				return []string{"1001", "2001"}, nil
			},
			wantUID:    1001,
			wantGID:    1001,
			wantGroups: []uint32{1001, 2001},
			wantErr:    false,
		},
		{
			name:     "UserNotFound",
			username: "unknown",
			fakeLookup: func(u string) (*user.User, error) {
				return nil, errors.New("user unknown")
			},
			wantErr: true,
		},
		{
			name:     "InvalidUID",
			username: "baduid",
			fakeLookup: func(u string) (*user.User, error) {
				return &user.User{Uid: "abc", Gid: "1001"}, nil
			},
			wantErr: true,
		},
		{
			name:     "InvalidGID",
			username: "badgid",
			fakeLookup: func(u string) (*user.User, error) {
				return &user.User{Uid: "1001", Gid: "abc"}, nil
			},
			wantErr: true,
		},
		{
			name:     "GroupLookupError",
			username: "badgroups",
			fakeLookup: func(u string) (*user.User, error) {
				return &user.User{Uid: "1001", Gid: "1001"}, nil
			},
			fakeGroupIds: func(u *user.User) ([]string, error) {
				return nil, errors.New("group lookup failed")
			},
			wantErr: true,
		},
		{
			name:     "InvalidGroupID",
			username: "badgroupid",
			fakeLookup: func(u string) (*user.User, error) {
				return &user.User{Uid: "1001", Gid: "1001"}, nil
			},
			fakeGroupIds: func(u *user.User) ([]string, error) {
				return []string{"1001", "abc"}, nil
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &osUserResolver{
				userLookup:   tt.fakeLookup,
				userGroupIds: tt.fakeGroupIds,
			}
			uid, gid, groups, err := p.lookupIDs(tt.username)

			if (err != nil) != tt.wantErr {
				t.Errorf("lookupIDs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if uid != tt.wantUID {
					t.Errorf("lookupIDs() uid = %v, want %v", uid, tt.wantUID)
				}
				if gid != tt.wantGID {
					t.Errorf("lookupIDs() gid = %v, want %v", gid, tt.wantGID)
				}
				if diff := cmp.Diff(tt.wantGroups, groups); diff != "" {
					t.Errorf("lookupIDs() groups mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestParseID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint32
		wantErr bool
	}{
		{
			name:    "ValidZero",
			input:   "0",
			want:    0,
			wantErr: false,
		},
		{
			name:    "ValidPositive",
			input:   "1000",
			want:    1000,
			wantErr: false,
		},
		{
			name:    "ValidMaxUint32",
			input:   "4294967295",
			want:    4294967295,
			wantErr: false,
		},
		{
			name:    "InvalidNegative",
			input:   "-1",
			want:    0,
			wantErr: true,
		},
		{
			name:    "InvalidNonNumeric",
			input:   "abc",
			want:    0,
			wantErr: true,
		},
		{
			name:    "InvalidOverflow",
			input:   "4294967296",
			want:    0,
			wantErr: true,
		},
		{
			name:    "InvalidEmpty",
			input:   "",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

type mockExecute struct {
	uidResult  Result
	gidResult  Result
	gidsResult Result
	t          *testing.T
}

func (m *mockExecute) Execute(ctx context.Context, params Params) Result {
	m.t.Helper()
	if params.Executable != "id" {
		m.t.Fatalf("unexpected executable: %s", params.Executable)
	}
	switch {
	case strings.HasPrefix(params.ArgsToSplit, "-u"):
		return m.uidResult
	case strings.HasPrefix(params.ArgsToSplit, "-g"):
		return m.gidResult
	case strings.HasPrefix(params.ArgsToSplit, "-G"):
		return m.gidsResult
	default:
		m.t.Fatalf("unexpected args: %s", params.ArgsToSplit)
		return Result{}
	}
}

func TestFetchIDs(t *testing.T) {
	tests := []struct {
		name       string
		uidResult  Result
		gidResult  Result
		gidsResult Result
		wantUID    uint32
		wantGID    uint32
		wantGroups []uint32
		wantErr    bool
	}{
		{
			name:       "Success",
			uidResult:  Result{StdOut: "1001\n"},
			gidResult:  Result{StdOut: "1002\n"},
			gidsResult: Result{StdOut: "1002 2001\n"},
			wantUID:    1001,
			wantGID:    1002,
			wantGroups: []uint32{1002, 2001},
			wantErr:    false,
		},
		{
			name:      "GetUIDFails",
			uidResult: Result{Error: errors.New("uid failed")},
			wantErr:   true,
		},
		{
			name:      "GetUIDBadOutput",
			uidResult: Result{StdOut: "abc\n"},
			wantErr:   true,
		},
		{
			name:      "GetGIDFails",
			uidResult: Result{StdOut: "1001\n"},
			gidResult: Result{Error: errors.New("gid failed")},
			wantErr:   true,
		},
		{
			name:      "GetGIDBadOutput",
			uidResult: Result{StdOut: "1001\n"},
			gidResult: Result{StdOut: "abc\n"},
			wantErr:   true,
		},
		{
			name:       "GetGroupsFails",
			uidResult:  Result{StdOut: "1001\n"},
			gidResult:  Result{StdOut: "1002\n"},
			gidsResult: Result{Error: errors.New("groups failed")},
			wantErr:    true,
		},
		{
			name:       "GetGroupsBadOutput",
			uidResult:  Result{StdOut: "1001\n"},
			gidResult:  Result{StdOut: "1002\n"},
			gidsResult: Result{StdOut: "1002 abc\n"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecute{
				uidResult:  tt.uidResult,
				gidResult:  tt.gidResult,
				gidsResult: tt.gidsResult,
				t:          t,
			}
			uid, gid, groups, err := fetchIDs(context.Background(), "testuser", mock.Execute)
			if (err != nil) != tt.wantErr {
				t.Errorf("fetchIDs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if uid != tt.wantUID {
					t.Errorf("fetchIDs() uid = %v, want %v", uid, tt.wantUID)
				}
				if gid != tt.wantGID {
					t.Errorf("fetchIDs() gid = %v, want %v", gid, tt.wantGID)
				}
				if diff := cmp.Diff(tt.wantGroups, groups); diff != "" {
					t.Errorf("fetchIDs() groups mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
