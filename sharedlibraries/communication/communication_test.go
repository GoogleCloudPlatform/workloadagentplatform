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

package communication

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	apb "google.golang.org/protobuf/types/known/anypb"
	"github.com/GoogleCloudPlatform/agentcommunication_client"
	acpb "github.com/GoogleCloudPlatform/agentcommunication_client/gapic/agentcommunicationpb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/gce/metadataserver"
	gpb "github.com/GoogleCloudPlatform/workloadagentplatform/sharedprotos/guestactions"
)

func wrapAny(t *testing.T, m proto.Message) *apb.Any {
	t.Helper()
	any, err := apb.New(m)
	if err != nil {
		t.Fatalf("anypb.New(%v) returned an unexpected error: %v", m, err)
	}
	return any
}

func TestListenForMessages(t *testing.T) {
	tests := []struct {
		name    string
		want    *acpb.MessageBody
		receive func(c *client.Connection) (*acpb.MessageBody, error)
	}{
		{
			name: "typical",
			want: &acpb.MessageBody{
				Labels: map[string]string{"uap_message_type": "OPERATION_STATUS", "operation_id": "test operation_id", "state": succeeded},
				Body:   &apb.Any{Value: []byte("typical test body")},
			},
			receive: func(c *client.Connection) (*acpb.MessageBody, error) {
				return &acpb.MessageBody{
					Labels: map[string]string{"uap_message_type": "OPERATION_STATUS", "operation_id": "test operation_id", "state": succeeded},
					Body:   &apb.Any{Value: []byte("typical test body")},
				}, nil
			},
		},
		{
			name: "emptyBody",
			want: &acpb.MessageBody{
				Labels: map[string]string{"uap_message_type": "OPERATION_STATUS", "operation_id": "test operation_id", "state": succeeded},
				Body:   &apb.Any{},
			},
			receive: func(c *client.Connection) (*acpb.MessageBody, error) {
				return &acpb.MessageBody{
					Labels: map[string]string{"uap_message_type": "OPERATION_STATUS", "operation_id": "test operation_id", "state": succeeded},
					Body:   &apb.Any{},
				}, nil
			},
		},
		{
			name: "noBody",
			want: &acpb.MessageBody{
				Labels: map[string]string{"uap_message_type": "OPERATION_STATUS", "operation_id": "test operation_id", "state": succeeded},
				Body:   nil,
			},
			receive: func(c *client.Connection) (*acpb.MessageBody, error) {
				return &acpb.MessageBody{
					Labels: map[string]string{"uap_message_type": "OPERATION_STATUS", "operation_id": "test operation_id", "state": succeeded},
					Body:   nil,
				}, nil
			},
		},
		{
			name: "emptyLabels",
			want: &acpb.MessageBody{
				Labels: map[string]string{},
				Body:   &apb.Any{Value: []byte("emptyLabels test body")},
			},
			receive: func(c *client.Connection) (*acpb.MessageBody, error) {
				return &acpb.MessageBody{
					Labels: map[string]string{},
					Body:   &apb.Any{Value: []byte("emptyLabels test body")},
				}, nil
			},
		},
		{
			name: "noLabels",
			want: &acpb.MessageBody{
				Body: &apb.Any{Value: []byte("noLabels test body")},
			},
			receive: func(c *client.Connection) (*acpb.MessageBody, error) {
				return &acpb.MessageBody{
					Body: &apb.Any{Value: []byte("noLabels test body")},
				}, nil
			},
		},
		{
			name: "noLabels",
			want: &acpb.MessageBody{
				Body: &apb.Any{Value: []byte("noLabels test body")},
			},
			receive: func(c *client.Connection) (*acpb.MessageBody, error) {
				return &acpb.MessageBody{
					Body: &apb.Any{Value: []byte("noLabels test body")},
				}, nil
			},
		},
		{
			name: "errorReceivingMessages",
			want: nil,
			receive: func(c *client.Connection) (*acpb.MessageBody, error) {
				return nil, fmt.Errorf("test error")
			},
		},
	}

	ctx := context.Background()
	conn := &client.Connection{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			receive = test.receive
			got := listenForMessages(ctx, conn, "testendpoint", "testchannel")
			if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("listenForMessages() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSendStatusMessage(t *testing.T) {
	tests := []struct {
		name        string
		want        error
		SendMessage func(c *client.Connection, msg *acpb.MessageBody) error
	}{
		{
			name: "typical",
			want: nil,
			SendMessage: func(c *client.Connection, msg *acpb.MessageBody) error {
				return nil
			},
		},
		{
			name: "errorSendingMessage",
			want: cmpopts.AnyError,
			SendMessage: func(c *client.Connection, msg *acpb.MessageBody) error {
				return fmt.Errorf("test error")
			},
		},
	}

	ctx := context.Background()
	conn := &client.Connection{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			origSendMessage := SendMessage
			defer func() { SendMessage = origSendMessage }()
			SendMessage = test.SendMessage
			got := SendStatusMessage(ctx, "operationID", &apb.Any{Value: []byte("test status body")}, "status", "lroState", conn)
			if diff := cmp.Diff(test.want, got, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("SendStatusMessage() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEstablishConnection(t *testing.T) {
	tests := []struct {
		name             string
		want             *client.Connection
		createConnection func(ctx context.Context, channel string, regional bool, opts ...option.ClientOption) (*client.Connection, error)
	}{
		{
			name: "typical",
			want: &client.Connection{},
			createConnection: func(ctx context.Context, channel string, regional bool, opts ...option.ClientOption) (*client.Connection, error) {
				return &client.Connection{}, nil
			},
		},
		{
			name: "error",
			want: nil,
			createConnection: func(ctx context.Context, channel string, regional bool, opts ...option.ClientOption) (*client.Connection, error) {
				return nil, cmpopts.AnyError
			},
		},
	}

	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			createConnection = test.createConnection
			got := establishConnection(ctx, "endpoint", "channel")
			if diff := cmp.Diff(test.want, got, protocmp.Transform(), cmpopts.IgnoreUnexported(client.Connection{})); diff != "" {
				t.Errorf("establishConnection() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEstablishACSConnection(t *testing.T) {
	tests := []struct {
		name             string
		want             *client.Connection
		createConnection func(ctx context.Context, channel string, regional bool, opts ...option.ClientOption) (*client.Connection, error)
	}{
		{
			name: "typical",
			want: &client.Connection{},
			createConnection: func(ctx context.Context, channel string, regional bool, opts ...option.ClientOption) (*client.Connection, error) {
				return &client.Connection{}, nil
			},
		},
		{
			name: "error",
			want: nil,
			createConnection: func(ctx context.Context, channel string, regional bool, opts ...option.ClientOption) (*client.Connection, error) {
				return nil, cmpopts.AnyError
			},
		},
	}

	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			createConnection = test.createConnection
			got := EstablishACSConnection(ctx, "endpoint", "channel")
			if diff := cmp.Diff(test.want, got, protocmp.Transform(), cmpopts.IgnoreUnexported(client.Connection{})); diff != "" {
				t.Errorf("EstablishACSConnection() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCommunicate(t *testing.T) {
	tests := []struct {
		name             string
		want             string
		createConnection func(ctx context.Context, channel string, regional bool, opts ...option.ClientOption) (*client.Connection, error)
		SendMessage      func(c *client.Connection, msg *acpb.MessageBody) error
		receive          func(c *client.Connection) (*acpb.MessageBody, error)
	}{
		{
			name: "typical",
			want: "",
			createConnection: func(ctx context.Context, channel string, regional bool, opts ...option.ClientOption) (*client.Connection, error) {
				return &client.Connection{}, nil
			},
			SendMessage: func(c *client.Connection, msg *acpb.MessageBody) error {
				return nil
			},
			receive: func(c *client.Connection) (*acpb.MessageBody, error) {
				time.Sleep(5 * time.Second)
				return &acpb.MessageBody{
					Labels: map[string]string{"uap_message_type": "OPERATION_STATUS", "operation_id": "test operation_id", "state": succeeded},
					Body: wrapAny(t, &gpb.GuestActionRequest{
						Commands: []*gpb.Command{
							{
								CommandType: &gpb.Command_ShellCommand{
									ShellCommand: &gpb.ShellCommand{Command: "echo", Args: "Hello World!"},
								},
							},
						},
					}),
				}, nil
			},
		},
		{
			name: "sendMessageError",
			want: "sendMessage error",
			createConnection: func(ctx context.Context, channel string, regional bool, opts ...option.ClientOption) (*client.Connection, error) {
				return &client.Connection{}, nil
			},
			SendMessage: func(c *client.Connection, msg *acpb.MessageBody) error {
				return fmt.Errorf("sendMessage error")
			},
			receive: func(c *client.Connection) (*acpb.MessageBody, error) {
				time.Sleep(5 * time.Second)
				return &acpb.MessageBody{
					Labels: map[string]string{"uap_message_type": "OPERATION_STATUS", "operation_id": "test operation_id", "state": succeeded},
					Body: wrapAny(t, &gpb.GuestActionRequest{
						Commands: []*gpb.Command{
							{
								CommandType: &gpb.Command_ShellCommand{
									ShellCommand: &gpb.ShellCommand{Command: "echo", Args: "Hello World!"},
								},
							},
						},
					}),
				}, nil
			},
		},
		{
			name: "receiveError",
			want: "nil labels in message from listenForMessages",
			createConnection: func(ctx context.Context, channel string, regional bool, opts ...option.ClientOption) (*client.Connection, error) {
				return &client.Connection{}, nil
			},
			SendMessage: func(c *client.Connection, msg *acpb.MessageBody) error {
				return nil
			},
			receive: func(c *client.Connection) (*acpb.MessageBody, error) {
				time.Sleep(5 * time.Second)
				return nil, fmt.Errorf("receive error")
			},
		},
		{
			name: "receiveOperationIdError",
			want: "no operation_id label",
			createConnection: func(ctx context.Context, channel string, regional bool, opts ...option.ClientOption) (*client.Connection, error) {
				return &client.Connection{}, nil
			},
			SendMessage: func(c *client.Connection, msg *acpb.MessageBody) error {
				return nil
			},
			receive: func(c *client.Connection) (*acpb.MessageBody, error) {
				time.Sleep(5 * time.Second)
				return &acpb.MessageBody{
					Labels: map[string]string{"uap_message_type": "OPERATION_STATUS", "state": succeeded},
				}, nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			createConnection = test.createConnection
			origSendMessage := SendMessage
			defer func() { SendMessage = origSendMessage }()
			SendMessage = test.SendMessage
			receive = test.receive
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			got := Communicate(ctx, "endpoint", "channel", func(context.Context, *apb.Any, *metadataserver.CloudProperties) (*apb.Any, error) {
				return nil, nil
			}, nil)
			if got == nil {
				if test.want != "" {
					t.Errorf("Communicate() returned nil error, want error: %s", test.want)
				}
				return
			}
			if test.want == "" {
				if got.Error() != "" {
					t.Errorf("Communicate() returned error: %s, want nil error", got.Error())
				}
				return
			}
			gotStr := strings.ReplaceAll(got.Error(), "\u00a0", " ")
			// Check if the desired error string is contained in the actual error string.
			if strings.Contains(gotStr, test.want) {
				return
			}
			// If the desired error substring is not present, give an error showing the diff.
			if diff := cmp.Diff(test.want, gotStr, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("Communicate() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListen(t *testing.T) {
	testMsg := &acpb.MessageBody{
		Labels: map[string]string{"test": "test"},
		Body:   &apb.Any{Value: []byte("test body")},
	}

	tests := []struct {
		name    string
		receive func() (*acpb.MessageBody, error)
		handler ConnectionHandler
		wantErr error
	}{
		{
			name: "handler error",
			receive: func() (*acpb.MessageBody, error) {
				return testMsg, nil
			},
			handler: func(ctx context.Context, msg *acpb.MessageBody, conn *client.Connection, cp *metadataserver.CloudProperties) error {
				return fmt.Errorf("handler error")
			},
			wantErr: ErrHandler,
		},
		{
			name: "receive error",
			receive: func() (*acpb.MessageBody, error) {
				return nil, fmt.Errorf("receive error")
			},
			handler: func(ctx context.Context, msg *acpb.MessageBody, conn *client.Connection, cp *metadataserver.CloudProperties) error {
				t.Fatal("handler should not be called when receive returns error")
				return nil
			},
			wantErr: ErrReceive,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			receive = func(c *client.Connection) (*acpb.MessageBody, error) {
				return tc.receive()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			err := Listen(ctx, &client.Connection{}, tc.handler, nil)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("Listen() returned error %v, want %v", err, tc.wantErr)
			}
		})
	}

	t.Run("handler called with success", func(t *testing.T) {
		handlerCalled := make(chan bool, 1)
		var receiveCount int
		receive = func(c *client.Connection) (*acpb.MessageBody, error) {
			receiveCount++
			if receiveCount == 1 {
				return testMsg, nil
			}
			return nil, fmt.Errorf("receive terminate")
		}
		handler := func(ctx context.Context, msg *acpb.MessageBody, conn *client.Connection, cp *metadataserver.CloudProperties) error {
			handlerCalled <- true
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		err := Listen(ctx, &client.Connection{}, handler, nil)

		select {
		case <-handlerCalled:
		default:
			t.Errorf("Listen() did not call handler")
		}
		if !errors.Is(err, ErrReceive) {
			t.Errorf("Listen() returned %v, want %v", err, ErrReceive)
		}
	})
}

func TestSendAgentMessage(t *testing.T) {
	tests := []struct {
		name        string
		want        error
		SendMessage func(c *client.Connection, msg *acpb.MessageBody) error
	}{
		{
			name: "typical",
			want: nil,
			SendMessage: func(c *client.Connection, msg *acpb.MessageBody) error {
				return nil
			},
		},
		{
			name: "errorSendingMessage",
			want: cmpopts.AnyError,
			SendMessage: func(c *client.Connection, msg *acpb.MessageBody) error {
				return fmt.Errorf("sendMessage error")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			origSendMessage := SendMessage
			defer func() { SendMessage = origSendMessage }()
			SendMessage = test.SendMessage
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			got := SendAgentMessage(ctx, "messageKey", "MessageType", &apb.Any{Value: []byte("test body")}, &client.Connection{})
			if diff := cmp.Diff(test.want, got, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("SendMessage() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}
