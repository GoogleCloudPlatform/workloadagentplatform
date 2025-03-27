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

package common

import (
	"testing"

	wpb "google.golang.org/protobuf/types/known/wrapperspb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/testing/protocmp"
	"go.uber.org/zap/zapcore"
	cpb "github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/protos"
	epb "github.com/GoogleCloudPlatform/workloadagentplatform/integration/example/protos"
)

// We are testing the Read functionality with a concrete example - the example agent configuration proto
func TestRead(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		readFunc ReadConfigFile
		config   proto.Message
		want     *epb.Configuration
		wantErr  bool
	}{
		{
			name: "ConfigFileWithContents",
			readFunc: func(p string) ([]byte, error) {
				fileContent := `{"log_to_cloud": false, "cloud_properties": {"project_id": "config-project-id", "instance_id": "config-instance-id", "zone": "config-zone" } }`
				return []byte(fileContent), nil
			},
			config: &epb.Configuration{},
			want: &epb.Configuration{
				CloudProperties: &cpb.CloudProperties{
					ProjectId:  "config-project-id",
					InstanceId: "config-instance-id",
					Zone:       "config-zone",
				},
				LogToCloud: &wpb.BoolValue{Value: false},
			},
		},
		{
			name: "MalformedFConfigurationJsonFile",
			readFunc: func(p string) ([]byte, error) {
				fileContent := `{"log_to_cloud": true, "cloud_properties": {"project_id": "config-project-id", "instance_id": "config-instance-id", "zone": "config-zone", } }`
				return []byte(fileContent), nil
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Read(test.path, test.readFunc, test.config)
			if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("ReadFromFile() for path: %s\n(-want +got):\n%s", test.path, diff)
			}
			if test.wantErr && err == nil {
				t.Errorf("ReadFromFile() for path: %s\nExpected error but got nil", test.path)
			}
		})
	}
}

func TestReadError(t *testing.T) {
	tests := []struct {
		name     string
		readFunc ReadConfigFile
		config   protoreflect.ProtoMessage
	}{
		{
			name: "FileReadError",
			readFunc: func(p string) ([]byte, error) {
				return nil, cmpopts.AnyError
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Read("", test.readFunc, test.config)
			if diff := cmp.Diff(cmpopts.AnyError, err); diff != "" {
				t.Errorf("ReadFromFile() error failure:\n(-want +got):\n%s", diff)
			}
		})
	}
}

func TestLogLevelToZapcore(t *testing.T) {
	tests := []struct {
		name  string
		level epb.Configuration_LogLevel
		want  zapcore.Level
	}{
		{
			name:  "INFO",
			level: epb.Configuration_INFO,
			want:  zapcore.InfoLevel,
		},
		{
			name:  "DEBUG",
			level: epb.Configuration_DEBUG,
			want:  zapcore.DebugLevel,
		},
		{
			name:  "WARNING",
			level: epb.Configuration_WARNING,
			want:  zapcore.WarnLevel,
		},
		{
			name:  "ERROR",
			level: epb.Configuration_ERROR,
			want:  zapcore.ErrorLevel,
		},
		{
			name:  "UNKNOWN",
			level: epb.Configuration_UNDEFINED,
			want:  zapcore.InfoLevel,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := LogLevelToZapcore(test.level.Number())
			if got != test.want {
				t.Errorf("LogLevelToZapcore(%v) = %v, want: %v", test.level, got, test.want)
			}
		})
	}
}
