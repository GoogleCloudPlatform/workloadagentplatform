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

package service

import (
	"context"
	"time"

	pb "github.com/GoogleCloudPlatform/workloadagentplatform/integration/example/protos"
	"github.com/GoogleCloudPlatform/workloadagentplatform/integration/example/usagemetrics"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"
)

// Slow implements the Service interface for the slow service.
type Slow struct {
	config *pb.Configuration
}

// Start starts the s;pw service.
func (s *Slow) Start(ctx context.Context, a any) {
	log.CtxLogger(ctx).Info("Starting Slow Service")
	for {
		log.CtxLogger(ctx).Info("Slow Service Start Loop")
		log.CtxLogger(ctx).Infof("Slow Service Waiting for %d seconds", s.config.GetSlowServiceLoopSeconds())
		select {
		case <-time.After(time.Duration(s.config.GetSlowServiceLoopSeconds()) * time.Second):
			log.CtxLogger(ctx).Info("Slow Service End Wait")
			s.doProcessing(ctx)
		case <-ctx.Done():
			log.CtxLogger(ctx).Info("Stopped Slow Service")
			return
		}
	}
}

// This is where the main processing for the service would be implemented.
func (s *Slow) doProcessing(ctx context.Context) {
	log.CtxLogger(ctx).Info("Slow Service Processing")
}

// String returns the name of the slow service.
func (s *Slow) String() string {
	return "Slow Service"
}

// ErrorCode returns the error code for the slow service.
func (s *Slow) ErrorCode() int {
	return usagemetrics.ExampleSlowServiceError
}

// ExpectedMinDuration returns the expected minimum duration for the slow service.
// Used by the recovery handler to determine if the service ran long enough to be considered
// successful.
func (s *Slow) ExpectedMinDuration() time.Duration {
	return 0
}
