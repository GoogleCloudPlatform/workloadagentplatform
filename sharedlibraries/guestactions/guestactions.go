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

// Package guestactions connects to the Agent Communication Service and handles guest actions in the agent.
// It acts as a dispatcher for commands sent by GCP services.
// The package supports both synchronous and asynchronous command execution.
// For asynchronous commands (LROs), it manages the lifecycle by sending
// intermediate "running" status updates followed by a final "done" status.
// For synchronous commands, it executes the action and sends a final "done" status.
package guestactions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/agentcommunication_client"
	"google.golang.org/protobuf/encoding/prototext"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/commandlineexecutor"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/communication"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/gce/metadataserver"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"

	anypb "google.golang.org/protobuf/types/known/anypb"
	acpb "github.com/GoogleCloudPlatform/agentcommunication_client/gapic/agentcommunicationpb"
	gpb "github.com/GoogleCloudPlatform/workloadagentplatform/sharedprotos/guestactions"
)

const (
	defaultEndpoint = ""
	agentCommand    = "agent_command"
	shellCommand    = "shell_command"

	statusSucceeded = "succeeded"
	statusFailed    = "failed"
	statusRunning   = "running"

	lroStateRunning = "running"
	lroStateDone    = "done"
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
	LROHandlers     map[string]GuestActionHandler
}

func anyResponse(ctx context.Context, gar *gpb.GuestActionResponse) *anypb.Any {
	any, err := anypb.New(gar)
	if err != nil {
		log.CtxLogger(ctx).Infow("Failed to marshal response to any", "err", err)
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
	log.CtxLogger(ctx).Debugw("Successfully unmarshalled message", "request_msg", prototext.Format(gaReq))
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
	log.CtxLogger(ctx).Debugw("Received result for shell command",
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
	handler, ok := g.options.LROHandlers[agentCommand]
	if !ok {
		handler, ok = g.options.Handlers[agentCommand]
		if !ok {
			errMsg := fmt.Sprintf("received unknown agent command: %s", prototext.Format(command))
			return &gpb.CommandResult{
				Command:  command,
				Stdout:   errMsg,
				Stderr:   errMsg,
				ExitCode: int32(1),
			}
		}
	}

	result := handler(ctx, command, cloudProperties)
	log.CtxLogger(ctx).Debugw("Received result for agent command", "result", prototext.Format(result))
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

// processCommands executes the commands and returns the results.
// It returns an error if any command fails or has a non-zero exit code.
func (g *GuestActions) processCommands(ctx context.Context, gar *gpb.GuestActionRequest, cloudProperties *metadataserver.CloudProperties) ([]*gpb.CommandResult, error) {
	var results []*gpb.CommandResult
	for _, command := range gar.GetCommands() {
		log.CtxLogger(ctx).Debugw("Processing command", "command", prototext.Format(command))
		pr := command.ProtoReflect()
		fd := pr.WhichOneof(pr.Descriptor().Oneofs().ByName("command_type"))
		result := &gpb.CommandResult{}
		switch {
		case fd == nil:
			errMsg := fmt.Sprintf("received unknown command: %s", prototext.Format(command))
			results = append(results, errorResult(errMsg))
			return results, errors.New(errMsg)
		case fd.Name() == shellCommand:
			result = handleShellCommand(ctx, command, commandlineexecutor.ExecuteCommand)
			results = append(results, result)
		case fd.Name() == agentCommand:
			result = g.handleAgentCommand(ctx, command, cloudProperties)
			results = append(results, result)
		default:
			errMsg := fmt.Sprintf("received unknown command: %s", prototext.Format(command))
			results = append(results, errorResult(errMsg))
			return results, errors.New(errMsg)
		}
		// Exit early if we get an error
		if result.GetExitCode() != int32(0) {
			errMsg := fmt.Sprintf("received nonzero exit code with output: %s", result.GetStdout())
			return results, errors.New(errMsg)
		}
	}
	return results, nil
}

func (g *GuestActions) executeAndSendDone(ctx context.Context, operationID string, gar *gpb.GuestActionRequest, conn *client.Connection, cloudProperties *metadataserver.CloudProperties) {
	results, err := g.processCommands(ctx, gar, cloudProperties)
	statusMsg := statusSucceeded
	errMsg := ""
	if err != nil {
		log.CtxLogger(ctx).Warnw("Failed to process commands", "operation_id", operationID, "channel", g.options.Channel, "err", err)
		statusMsg = statusFailed
		errMsg = err.Error()
	}
	// Send final status
	err = communication.SendStatusMessage(ctx, operationID, anyResponse(ctx, guestActionResponse(ctx, results, errMsg)), statusMsg, lroStateDone, conn)
	if err != nil {
		log.CtxLogger(ctx).Warnw("SendStatusMessage failed", "operation_id", operationID, "channel", g.options.Channel, "err", err)
	}
}

func (g *GuestActions) isLRORequest(gaReq *gpb.GuestActionRequest) bool {
	for _, command := range gaReq.GetCommands() {
		if ac := command.GetAgentCommand(); ac != nil {
			if _, ok := g.options.LROHandlers[strings.ToLower(ac.GetCommand())]; ok {
				return true
			}
		}
	}
	return false
}

// connectionHandler parses incoming messages from ACS and dispatches them to the appropriate command handlers.
// If a message contains an asynchronous command, it sends an initial "running" status message,
// processes the command in a goroutine, and sends a final "done" status message upon completion.
// Synchronous commands are processed in a separate goroutine to avoid blocking the listener loop,
// but no initial "running" status is sent.
func (g *GuestActions) connectionHandler(ctx context.Context, msg *acpb.MessageBody, conn *client.Connection, cloudProperties *metadataserver.CloudProperties) error {
	if msg.GetLabels() == nil {
		err := errors.New("received message with nil labels")
		log.CtxLogger(ctx).Warnw("Connection handler failed", "err", err)
		return err
	}
	operationID, ok := msg.GetLabels()["operation_id"]
	if !ok {
		err := errors.New("received message with no operation_id in labels")
		log.CtxLogger(ctx).Warnw("Connection handler failed", "err", err)
		return err
	}
	gaReq, err := parseRequest(ctx, msg.GetBody())
	if err != nil {
		log.CtxLogger(ctx).Warnw("Failed to parse request", "operation_id", operationID, "channel", g.options.Channel, "err", err)
		return err
	}
	log.CtxLogger(ctx).Debugw("Received GuestActionRequest", "operation_id", operationID, "channel", g.options.Channel, "request_msg", prototext.Format(gaReq))

	if g.isLRORequest(gaReq) {
		// Send initial running status
		err := communication.SendStatusMessage(ctx, operationID, anyResponse(ctx, guestActionResponse(ctx, nil, "")), statusRunning, lroStateRunning, conn)
		if err != nil {
			log.CtxLogger(ctx).Warnw("SendStatusMessage failed", "operation_id", operationID, "channel", g.options.Channel, "err", err)
			return err
		}
	}

	// Process commands in background to avoid blocking the listener loop.
	go g.executeAndSendDone(ctx, operationID, gaReq, conn, cloudProperties)
	return nil
}

// Start starts listening to ACS/UAP and handling the related guest actions.
func (g *GuestActions) Start(ctx context.Context, a any) {
	args, ok := a.(Options)
	if !ok {
		log.CtxLogger(ctx).Warn("Args is not of type Options")
		return
	}
	g.options = args
	endpoint := defaultEndpoint
	if g.options.Endpoint != "" {
		endpoint = g.options.Endpoint
	}
	log.CtxLogger(ctx).Debugw("Listening for ACS messages", "endpoint", endpoint, "channel", args.Channel)
	conn := communication.EstablishACSConnection(ctx, endpoint, args.Channel)
	if conn == nil {
		log.CtxLogger(ctx).Errorw("Failed to establish ACS connection, exiting", "endpoint", endpoint, "channel", args.Channel)
		return
	}
	if err := communication.Listen(ctx, conn, g.connectionHandler, args.CloudProperties); err != nil {
		log.CtxLogger(ctx).Errorw("Failed to listen for ACS messages, exiting", "err", err, "endpoint", endpoint, "channel", args.Channel)
		return
	}
}
