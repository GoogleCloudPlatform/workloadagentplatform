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

// Package communication provides capability for Google Cloud Agents to communicate with
// Google Cloud Service Providers via Agent Communication Service (ACS).
// Messages received will typically have been sent via UAP Communication Highway.
package communication

import (
	"context"
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/agentcommunication_client"
	"github.com/cenkalti/backoff/v4"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/prototext"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/gce/metadataserver"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"

	anypb "google.golang.org/protobuf/types/known/anypb"
	acpb "github.com/GoogleCloudPlatform/agentcommunication_client/gapic/agentcommunicationpb"
)

const (
	succeeded = "SUCCEEDED"
	failed    = "FAILED"
)

type (
	// MsgHandlerFunc is the function that the agent will use to handle incoming messages.
	MsgHandlerFunc func(context.Context, *anypb.Any, *metadataserver.CloudProperties) (*anypb.Any, error)
)

var sendMessage = func(c *client.Connection, msg *acpb.MessageBody) error {
	return c.SendMessage(msg)
}

var receive = func(c *client.Connection) (*acpb.MessageBody, error) {
	return c.Receive()
}

var createConnection = func(ctx context.Context, channel string, regional bool, opts ...option.ClientOption) (*client.Connection, error) {
	acsClient, err := client.NewClient(ctx, regional, opts...)
	if err != nil {
		return nil, err
	}
	return client.NewConnection(ctx, channel, acsClient)
}

func sendStatusMessage(ctx context.Context, operationID string, body *anypb.Any, status string, conn *client.Connection) error {
	labels := map[string]string{
		"operation_id": operationID,
		"state":        status,
		"lro_state":    "done",
	}
	messageToSend := &acpb.MessageBody{Labels: labels, Body: body}
	log.CtxLogger(ctx).Debugw("Sending status message via ACS.", "messageToSend", messageToSend)
	if err := sendMessage(conn, messageToSend); err != nil {
		return fmt.Errorf("error sending status message via ACS: %v", err)
	}
	return nil
}

func listenForMessages(ctx context.Context, conn *client.Connection, endpoint string, channel string) *acpb.MessageBody {
	log.CtxLogger(ctx).Debugw("Listening for messages on ACS.", "endpoint", endpoint, "channel", channel)
	msg, err := receive(conn)
	if err != nil {
		log.CtxLogger(ctx).Warn(err)
		return nil
	}
	log.CtxLogger(ctx).Debugw("ACS Message received.", "msg", msg)
	return msg
}

func establishConnection(ctx context.Context, endpoint string, channel string) *client.Connection {
	log.CtxLogger(ctx).Infow("Establishing connection with ACS.", "endpoint", endpoint, "channel", channel)
	var opts []option.ClientOption
	if endpoint != "" {
		log.CtxLogger(ctx).Infow("Using non-default endpoint.", "endpoint", endpoint)
		opts = append(opts, option.WithEndpoint(endpoint))
	}
	conn, err := createConnection(ctx, channel, true, opts...)
	if err != nil {
		log.CtxLogger(ctx).Warnw("Failed to establish connection to ACS.", "err", err)
	}
	log.CtxLogger(ctx).Info("Connected to ACS.")
	return conn
}

func setupBackoff() backoff.BackOff {
	b := &backoff.ExponentialBackOff{
		InitialInterval:     2 * time.Second,
		RandomizationFactor: 0,
		Multiplier:          2,
		MaxInterval:         1 * time.Hour,
		MaxElapsedTime:      0,
		Clock:               backoff.SystemClock,
	}
	b.Reset()
	return b
}

func logAndBackoff(ctx context.Context, eBackoff backoff.BackOff, msg string) {
	duration := eBackoff.NextBackOff()
	log.CtxLogger(ctx).Infow(msg, "duration", duration)
	time.Sleep(duration)
}

// Communicate establishes ongoing communication with ACS.
// "endpoint" is the endpoint and will often be an empty string.
// "channel" is the registered channel name to be used for communication
// between the agent and the service provider.
// "messageHandler" is the function that the agent will use to handle incoming messages.
func Communicate(ctx context.Context, endpoint string, channel string, messageHandler MsgHandlerFunc, cloudProperties *metadataserver.CloudProperties) error {
	eBackoff := setupBackoff()
	conn := establishConnection(ctx, endpoint, channel)
	for conn == nil {
		logMsg := fmt.Sprintf("Establishing connection failed. Will backoff and retry.")
		logAndBackoff(ctx, eBackoff, logMsg)
		conn = establishConnection(ctx, endpoint, channel)
	}
	// Reset backoff once we successfully connected.
	eBackoff.Reset()

	var lastErr error
	for {
		// Return most recent error if context is cancelled. Useful for unit testing purposes.
		select {
		case <-ctx.Done():
			log.CtxLogger(ctx).Info("Context is done. Returning.")
			return lastErr
		default:
		}
		// listen for messages
		msg := listenForMessages(ctx, conn, endpoint, channel)
		log.CtxLogger(ctx).Infow("ListenForMessages complete.", "msg", prototext.Format(msg))
		// parse message
		if msg.GetLabels() == nil {
			logMsg := fmt.Sprintf("Nil labels in message from listenForMessages. Will backoff and retry with a new connection.")
			logAndBackoff(ctx, eBackoff, logMsg)
			conn = establishConnection(ctx, endpoint, channel)
			lastErr = fmt.Errorf("nil labels in message from listenForMessages")
			continue
		}
		operationID, ok := msg.GetLabels()["operation_id"]
		if !ok {
			logMsg := fmt.Sprintf("No operation_id label in message. Will backoff and retry.")
			logAndBackoff(ctx, eBackoff, logMsg)
			lastErr = fmt.Errorf("no operation_id label in message")
			continue
		}
		log.CtxLogger(ctx).Debugw("Parsed operation_id from label.", "operation_id", operationID)
		// Reset backoff if we successfully parsed the message.
		eBackoff.Reset()
		// handle the message
		res, err := messageHandler(ctx, msg.GetBody(), cloudProperties)
		statusMsg := succeeded
		if err != nil {
			log.CtxLogger(ctx).Warnw("Encountered error during ACS message handling.", "err", err)
			statusMsg = failed
		}
		log.CtxLogger(ctx).Debugw("Message handling complete.", "responseMsg", prototext.Format(res), "statusMsg", statusMsg)
		// Send operation status message.
		err = sendStatusMessage(ctx, operationID, res, statusMsg, conn)
		if err != nil {
			log.CtxLogger(ctx).Warnw("Encountered error during sendStatusMessage.", "err", err)
			lastErr = err
		}
	}
}

// EstablishACSConnection establishes a connection to ACS.
// "endpoint" is the endpoint and will often be an empty string.
// "channel" is the registered channel name to be used for communication
// between the agent and the service provider.
func EstablishACSConnection(ctx context.Context, endpoint string, channel string) *client.Connection {
	return establishConnection(ctx, endpoint, channel)
}

// SendAgentMessage sends a message from the agent to the service provider via ACS
// "messageKey" is the label key to be used for the message.
// "messageType" is the label value of the message.
// "body" is the body of the message.
// "conn" is the connection to ACS.
func SendAgentMessage(ctx context.Context, messageKey string, messageType string, body *anypb.Any, conn *client.Connection) error {
	labels := map[string]string{
		messageKey: messageType,
	}
	messageToSend := &acpb.MessageBody{Labels: labels, Body: body}
	log.CtxLogger(ctx).Debugw("Sending agent message via ACS.", "messageToSend", messageToSend)
	if err := sendMessage(conn, messageToSend); err != nil {
		return fmt.Errorf("error sending agent message via ACS: %v", err)
	}
	return nil
}
