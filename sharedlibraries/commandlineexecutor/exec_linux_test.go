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
	"errors"
	"os/user"
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
