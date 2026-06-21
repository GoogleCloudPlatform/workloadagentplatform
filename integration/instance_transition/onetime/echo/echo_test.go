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

package echo

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNewEcho(t *testing.T) {
	tests := []struct {
		name, text string
		e          *Echo
		want       error
	}{
		{
			name: "SuccessNilMessage",
			e:    &Echo{},
		},
		{
			name: "SuccessEmptyMessage",
			e:    &Echo{text: ""},
		},
		{
			name: "SuccessWithMessage",
			e:    &Echo{text: "hello"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.e.Execute(&cobra.Command{})
			if got != test.want {
				t.Errorf("Execute(%v)=%v, want %v", test.e, got, test.want)
			}
		})
	}
}
