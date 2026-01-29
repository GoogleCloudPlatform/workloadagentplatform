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
	"os/exec"
)

// osUserResolver is a no-op wrapper for Windows.
type osUserResolver struct{}

// newOSUserResolver returns a no-op userResolver.
func newOSUserResolver() userResolver {
	return &osUserResolver{}
}

// lookupIDs is a no-op on Windows.
func (o *osUserResolver) lookupIDs(username string) (uint32, uint32, []uint32, error) {
	return 0, 0, nil, nil
}

// setupExeForPlatform is not implemented for windows.
func setupExeForPlatform(ctx context.Context, exe *exec.Cmd, params Params, executeCommand Execute, resolver userResolver) error {
	return nil
}
