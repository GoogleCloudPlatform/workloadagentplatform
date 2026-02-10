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
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

// osUserResolver is a wrapper around the os/user package for testability.
type osUserResolver struct {
	userLookup   func(username string) (*user.User, error)
	userGroupIds func(u *user.User) ([]string, error)
}

// newOSUserResolver returns a userResolver that uses the real os/user package.
func newOSUserResolver() userResolver {
	return &osUserResolver{
		userLookup:   user.Lookup,
		userGroupIds: func(u *user.User) ([]string, error) { return u.GroupIds() },
	}
}

// setupExeForPlatform sets up the env and user if provided in the params.
// returns an error if it could not be setup
func setupExeForPlatform(ctx context.Context, exe *exec.Cmd, params Params, executeCommand Execute, userResolver userResolver) error {
	// set the execution environment if params Env exists
	if len(params.Env) > 0 {
		exe.Env = append(exe.Environ(), params.Env...)
	}

	// if params.User exists run as the user
	if params.User != "" {
		uid, gid, groups, err := userResolver.lookupIDs(params.User)
		if err != nil {
			return err
		}
		exe.SysProcAttr = &syscall.SysProcAttr{}
		exe.SysProcAttr.Credential = &syscall.Credential{Uid: uid, Gid: gid, Groups: groups}
	}
	return nil
}

func (o *osUserResolver) lookupIDs(username string) (uint32, uint32, []uint32, error) {
	u, err := o.userLookup(username)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("lookup user: %w", err)
	}

	uid, err := parseID(u.Uid)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("parse uid: %w", err)
	}

	gid, err := parseID(u.Gid)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("parse gid: %w", err)
	}

	groupStrIDs, err := o.userGroupIds(u)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("lookup groups: %w", err)
	}

	var groups []uint32
	for _, gID := range groupStrIDs {
		g, err := parseID(gID)
		if err != nil {
			return 0, 0, nil, fmt.Errorf("parse group id: %w", err)
		}
		groups = append(groups, g)
	}

	return uid, gid, groups, nil
}

/*
fetchIDs fetches the UID, GID, and Group IDs for the given user.
This is done by executing the "id" command on the system.
Note: This is intended for Linux based system only.
*/
func fetchIDs(ctx context.Context, user string, executeCommand Execute) (uint32, uint32, []uint32, error) {
	result := executeCommand(ctx, Params{
		Executable:  "id",
		ArgsToSplit: fmt.Sprintf("-u %s", user),
	})
	if result.Error != nil {
		return 0, 0, nil, fmt.Errorf("getUID failed with: %s. StdErr: %s", result.Error, result.StdErr)
	}
	uid, err := strconv.Atoi(strings.TrimSuffix(result.StdOut, "\n"))
	if err != nil {
		return 0, 0, nil, fmt.Errorf("could not parse UID from StdOut: %s", result.StdOut)
	}

	result = executeCommand(ctx, Params{
		Executable:  "id",
		ArgsToSplit: fmt.Sprintf("-g %s", user),
	})
	if result.Error != nil {
		return 0, 0, nil, fmt.Errorf("getGID failed with: %s. StdErr: %s", result.Error, result.StdErr)
	}
	gid, err := strconv.Atoi(strings.TrimSuffix(result.StdOut, "\n"))
	if err != nil {
		return 0, 0, nil, fmt.Errorf("could not parse GID from StdOut: %s", result.StdOut)
	}

	result = executeCommand(ctx, Params{
		Executable:  "id",
		ArgsToSplit: fmt.Sprintf("-G %s", user),
	})
	if result.Error != nil {
		return 0, 0, nil, fmt.Errorf("getGroups failed with: %s. StdErr: %s", result.Error, result.StdErr)
	}
	groups := strings.Split(strings.TrimSuffix(result.StdOut, "\n"), " ")
	var groupIDs []uint32
	for _, gID := range groups {
		g, err := strconv.Atoi(gID)
		if err != nil {
			return 0, 0, nil, fmt.Errorf("could not parse GroupID from StdOut: %s", result.StdOut)
		}
		groupIDs = append(groupIDs, uint32(g))
	}
	return uint32(uid), uint32(gid), groupIDs, nil
}

// Helper to convert string IDs to uint32.
func parseID(id string) (uint32, error) {
	val, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(val), nil
}
