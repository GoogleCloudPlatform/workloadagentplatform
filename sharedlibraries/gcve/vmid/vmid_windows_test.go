//go:build windows

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

package vmid

import (
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestGetVMID(t *testing.T) {
	testCases := []struct {
		name        string
		queryErr    error
		queryResult []win32_BIOS
		wantVMID    string
		wantErr     bool
	}{
		{
			name:        "valid_serial_number",
			queryResult: []win32_BIOS{win32_BIOS{SerialNumber: "VMware-01 23 45 67 89 ab cd ef-01 23 45 67 89 ab cd ef"}},
			wantVMID:    "01234567-89ab-cdef-0123-456789abcdef",
			wantErr:     false,
		},
		{
			name:     "query_error",
			queryErr: cmpopts.AnyError,
			wantVMID: "",
			wantErr:  true,
		},
		{
			name:        "empty_query_result",
			queryResult: []win32_BIOS{},
			wantVMID:    "",
			wantErr:     true,
		},
		{
			name:        "empty_serial_number",
			queryResult: []win32_BIOS{win32_BIOS{}},
			wantVMID:    "",
			wantErr:     true,
		},
		{
			name:        "unexpected_prefix",
			queryResult: []win32_BIOS{win32_BIOS{SerialNumber: "GoogleCloud-0123456789ABCDEF0123456789ABCDEF"}},
			wantVMID:    "",
			wantErr:     true,
		},
		{
			name:        "parsing_error",
			queryResult: []win32_BIOS{win32_BIOS{SerialNumber: "VMware-01 23 45 67 89 ab cd ef-01 23"}},
			wantVMID:    "",
			wantErr:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			queryBIOS = func() ([]win32_BIOS, error) {
				return tc.queryResult, nil
			}

			// Execute
			gotVMID, err := VMID()

			// Validate
			if err != nil && !tc.wantErr {
				t.Errorf("VMID() returned an unexpected error: %v", err)
			}
			if err == nil && tc.wantErr {
				t.Errorf("VMID() did not return an error, but an error was expected")
			}
			if gotVMID != tc.wantVMID {
				t.Errorf("VMID() = %s, want: %s", gotVMID, tc.wantVMID)
			}
		})
	}
}
