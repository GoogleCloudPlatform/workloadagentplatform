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

// Fast implements the Service interface for the fast service.
type Fast struct {
	config *pb.Configuration
}

// Start starts the fast service.
func (s *Fast) Start(ctx context.Context, a any) {
	log.CtxLogger(ctx).Info("Starting Fast Service")
	for {
		log.CtxLogger(ctx).Info("Fast Service Start Loop")
		log.CtxLogger(ctx).Infof("Fast Service Waiting for %d seconds", s.config.GetFastServiceLoopSeconds())
		select {
		case <-time.After(time.Duration(s.config.GetFastServiceLoopSeconds()) * time.Second):
			log.CtxLogger(ctx).Info("Fast Service End Wait")
			s.doProcessing(ctx)
		case <-ctx.Done():
			log.CtxLogger(ctx).Info("Stopped Fast Service")
			return
		}
	}
}

// This is where the main processing for the service would be implemented.
func (s *Fast) doProcessing(ctx context.Context) {
	log.CtxLogger(ctx).Info("Fast Service Processing")
}

// String returns the name of the fast service.
func (s *Fast) String() string {
	return "Fast Service"
}

// ErrorCode returns the error code for the fast service.
func (s *Fast) ErrorCode() int {
	return usagemetrics.ExampleFastServiceError
}

// ExpectedMinDuration returns the expected minimum duration for the fast service.
// Used by the recovery handler to determine if the service ran long enough to be considered
// successful.
func (s *Fast) ExpectedMinDuration() time.Duration {
	return 0
}
