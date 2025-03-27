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

// Package common provides common platform integration code.
package common

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"go.uber.org/zap/zapcore"
	"github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/usagemetrics"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"
)

// ReadConfigFile abstracts os.ReadFile function for testability.
type ReadConfigFile func(string) ([]byte, error)

// Read just reads configuration from given file and parses it into config proto.
// This should be called using the specific protocol buffer message for the config, example:
//
//	configuration.Read(d.configFilePath, os.ReadFile, &pb.Configuration{})
func Read(path string, read ReadConfigFile, config proto.Message) (proto.Message, error) {
	content, err := read(path)
	if err != nil || len(content) == 0 {
		log.Logger.Errorw("Could not read from configuration file", "file", path, "error", err)
		usagemetrics.Error(usagemetrics.GlobalConfigFileReadError)
		return nil, err
	}

	err = protojson.Unmarshal(content, config)
	if err != nil {
		usagemetrics.Error(usagemetrics.GlobalMalformedConfigFileError)
		log.Logger.Errorw("Invalid content in the configuration file", "file", path, "content", string(content))
		log.Logger.Errorf("Configuration JSON at '%s' has error: %v.  Please fix the JSON and restart the agent", path, err)
		return nil, err
	}
	return config, nil
}

// LogLevelToZapcore returns the zapcore equivalent of the configuration log level.
func LogLevelToZapcore(level protoreflect.EnumNumber) zapcore.Level {
	switch level {
	case 1: // DEBUG
		return zapcore.DebugLevel
	case 2: // INFO
		return zapcore.InfoLevel
	case 3: // WARNING
		return zapcore.WarnLevel
	case 4: // ERROR
		return zapcore.ErrorLevel
	default:
		log.Logger.Warnw("Unsupported log level, defaulting to INFO", "level", level)
		return zapcore.InfoLevel
	}
}
