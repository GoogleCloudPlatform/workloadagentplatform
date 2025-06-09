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
			queryResult: []win32_BIOS{win32_BIOS{SerialNumber: "VMware-12345678901234567890123456789012"}},
			wantVMID:    "12345678-9012-3456-7890-123456789012",
			wantErr:     false,
		},
		{
			name:        "invalid_serial_number",
			queryResult: []win32_BIOS{win32_BIOS{SerialNumber: "12345678901234567890123456789012"}},
			wantVMID:    "",
			wantErr:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			queryBIOS = func() ([]win32_BIOS, error) {
				return tc.queryResult, nil
			}
			gotVMID, err := VMID()
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
