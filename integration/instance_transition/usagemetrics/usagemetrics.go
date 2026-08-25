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

// Package usagemetrics provides usage metric constants for the instance_transition integration.
package usagemetrics

// Error codes for instance_transition integration.  Integration error codes should start from 51.
// Always add the error code to the end of the list and do not modify existing codes.
// All errors should be prefixed with the integration name.
const (
	InstanceTransitionFastServiceError = 51
	InstanceTransitionSlowServiceError = 52
)

// Action codes for instance_transition integration.  Integration action codes should start from 51.
// Always add the action code to the end of the list and do not modify existing codes.
// All actions should be prefixed with the integration name.
const (
	InstanceTransitionEchoStarted  = 51
	InstanceTransitionEchoFinished = 52
)
