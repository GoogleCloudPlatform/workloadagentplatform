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

// Package guestactions connects to Agent Communication Service (ACS) and handles guest actions in
// the agent. Messages received via ACS will typically have been sent via UAP Communication Highway.
package guestactions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	anypb "google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/encoding/prototext"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/commandlineexecutor"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/communication"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/gce/metadataserver"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"
	gpb "github.com/GoogleCloudPlatform/workloadagentplatform/sharedprotos/guestactions"
)

const (
	defaultEndpoint = ""
	agentCommand    = "agent_command"
	shellCommand    = "shell_command"
)

// GuestActions is a struct that holds the state for guest actions.
type GuestActions struct {
	options Options
}

// GuestActionHandler is a function that handles a guest action command.
type GuestActionHandler func(context.Context, *gpb.Command, *metadataserver.CloudProperties) *gpb.CommandResult

// Options is a struct that holds the options for guest actions.
type Options struct {
	Channel         string
	Endpoint        string
	CloudProperties *metadataserver.CloudProperties
	Handlers        map[string]GuestActionHandler
}

func anyResponse(ctx context.Context, gar *gpb.GuestActionResponse) *anypb.Any {
	any, err := anypb.New(gar)
	if err != nil {
		log.CtxLogger(ctx).Infow("Failed to marshal response to any.", "err", err)
		any = &anypb.Any{}
	}
	return any
}

func parseRequest(ctx context.Context, msg *anypb.Any) (*gpb.GuestActionRequest, error) {
	gaReq := &gpb.GuestActionRequest{}
	if err := msg.UnmarshalTo(gaReq); err != nil {
		errMsg := fmt.Sprintf("failed to unmarshal message: %v", err)
		return nil, errors.New(errMsg)
	}
	log.CtxLogger(ctx).Debugw("successfully unmarshalled message.", "gar", prototext.Format(gaReq))
	return gaReq, nil
}

func guestActionResponse(ctx context.Context, results []*gpb.CommandResult, errorMessage string) *gpb.GuestActionResponse {
	return &gpb.GuestActionResponse{
		CommandResults: results,
		Error: &gpb.GuestActionError{
			ErrorMessage: errorMessage,
		},
	}
}

func handleShellCommand(ctx context.Context, command *gpb.Command, execute commandlineexecutor.Execute) *gpb.CommandResult {
	sc := command.GetShellCommand()
	result := execute(
		ctx,
		commandlineexecutor.Params{
			Executable:  sc.GetCommand(),
			ArgsToSplit: sc.GetArgs(),
			Timeout:     int(sc.GetTimeoutSeconds()),
		},
	)
	log.CtxLogger(ctx).Debugw("received result for shell command.",
		"command", prototext.Format(command), "stdOut", result.StdOut,
		"stdErr", result.StdErr, "error", result.Error, "exitCode", result.ExitCode)
	exitCode := int32(result.ExitCode)
	if exitCode == 0 && (result.Error != nil || result.StdErr != "") {
		exitCode = int32(1)
	}
	return &gpb.CommandResult{
		Command:  command,
		Stdout:   result.StdOut,
		Stderr:   result.StdErr,
		ExitCode: exitCode,
	}
}

func (g *GuestActions) handleAgentCommand(ctx context.Context, command *gpb.Command, cloudProperties *metadataserver.CloudProperties) *gpb.CommandResult {
	agentCommand := strings.ToLower(command.GetAgentCommand().GetCommand())
	handler, ok := g.options.Handlers[agentCommand]
	if !ok {
		errMsg := fmt.Sprintf("received unknown agent command: %s", prototext.Format(command))
		result := &gpb.CommandResult{
			Command:  command,
			Stdout:   errMsg,
			Stderr:   errMsg,
			ExitCode: int32(1),
		}
		return result
	}
	result := handler(ctx, command, cloudProperties)
	log.CtxLogger(ctx).Debugw("received result for agent command.", "result", prototext.Format(result))
	return result
}

func errorResult(errMsg string) *gpb.CommandResult {
	return &gpb.CommandResult{
		Command:  nil,
		Stdout:   errMsg,
		Stderr:   errMsg,
		ExitCode: int32(1),
	}
}

func (g *GuestActions) messageHandler(ctx context.Context, req *anypb.Any, cloudProperties *metadataserver.CloudProperties) (*anypb.Any, error) {
	var results []*gpb.CommandResult
	gar, err := parseRequest(ctx, req)
	if err != nil {
		log.CtxLogger(ctx).Debugw("failed to parse request.", "err", err)
		return nil, err
	}
	log.CtxLogger(ctx).Debugw("received GuestActionRequest to handle", "gar", prototext.Format(gar))
	for _, command := range gar.GetCommands() {
		log.CtxLogger(ctx).Debugw("processing command.", "command", prototext.Format(command))
		pr := command.ProtoReflect()
		fd := pr.WhichOneof(pr.Descriptor().Oneofs().ByName("command_type"))
		result := &gpb.CommandResult{}
		switch {
		case fd == nil:
			errMsg := fmt.Sprintf("received unknown command: %s", prototext.Format(command))
			results = append(results, errorResult(errMsg))
			return anyResponse(ctx, guestActionResponse(ctx, results, errMsg)), errors.New(errMsg)
		case fd.Name() == shellCommand:
			result = handleShellCommand(ctx, command, commandlineexecutor.ExecuteCommand)
			results = append(results, result)
		case fd.Name() == agentCommand:
			result = g.handleAgentCommand(ctx, command, cloudProperties)
			results = append(results, result)
		default:
			errMsg := fmt.Sprintf("received unknown command: %s", prototext.Format(command))
			results = append(results, errorResult(errMsg))
			return anyResponse(ctx, guestActionResponse(ctx, results, errMsg)), errors.New(errMsg)
		}
		// Exit early if we get an error
		if result.GetExitCode() != int32(0) {
			errMsg := fmt.Sprintf("received nonzero exit code with output: %s", result.GetStdout())
			return anyResponse(ctx, guestActionResponse(ctx, results, errMsg)), errors.New(errMsg)
		}
	}
	return anyResponse(ctx, guestActionResponse(ctx, results, "")), nil
}

// Start starts listening to ACS/UAP and handling the related guest actions.
func (g *GuestActions) Start(ctx context.Context, a any) {
	args, ok := a.(Options)
	if !ok {
		log.CtxLogger(ctx).Warn("args is not of type guestActionsArgs")
		return
	}
	g.options = args
	endpoint := defaultEndpoint
	if g.options.Endpoint != "" {
		endpoint = g.options.Endpoint
	}
	communication.Communicate(ctx, endpoint, args.Channel, g.messageHandler, args.CloudProperties)
}
