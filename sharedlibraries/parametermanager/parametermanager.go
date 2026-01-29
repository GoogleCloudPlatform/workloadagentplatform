/*
Copyright 2026 Google LLC

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

// Package parametermanager provides functionality to fetch configuration from Google Cloud Parameter Manager.
package parametermanager

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"

	"google.golang.org/api/option"
	parametermanagerpb "google.golang.org/api/parametermanager/v1"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"
)

// Resource represents a fetched parameter configuration.
type Resource struct {
	Data    string
	Version string
}

// Client wraps the Parameter Manager service.
type Client struct {
	service *parametermanagerpb.Service
}

// NewClient creates a new Parameter Manager client.
func NewClient(ctx context.Context, opts ...option.ClientOption) (*Client, error) {
	service, err := parametermanagerpb.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	return &Client{service: service}, nil
}

// FetchParameter is a high-level function to fetch a parameter.
// It resolves the latest version if version is empty.
func (c *Client) FetchParameter(ctx context.Context, projectID, location, parameterName, version string) (*Resource, error) {
	// Construct the full parameter name
	// Format: projects/{project}/locations/{location}/parameters/{parameter_name}
	name := fmt.Sprintf("projects/%s/locations/%s/parameters/%s", projectID, location, parameterName)

	var targetVersion *parametermanagerpb.ParameterVersion
	var err error

	if version != "" {
		targetVersion, err = c.fetchSpecificVersion(ctx, name, version)
	} else {
		targetVersion, err = c.fetchLatestVersion(ctx, name)
	}
	if err != nil {
		return nil, err
	}

	return c.renderVersion(ctx, targetVersion)
}

// fetchSpecificVersion fetches metadata for a specific version.
func (c *Client) fetchSpecificVersion(ctx context.Context, paramName, version string) (*parametermanagerpb.ParameterVersion, error) {
	versionName := fmt.Sprintf("%s/versions/%s", paramName, version)
	log.CtxLogger(ctx).Infow("Fetching specific parameter version", "versionName", versionName)

	resp, err := c.service.Projects.Locations.Parameters.Versions.Get(versionName).Do()
	if err != nil {
		log.Logger.Errorw("Failed to get parameter version", "versionName", versionName, "error", err)
		return nil, fmt.Errorf("failed to get parameter version: %w", err)
	}
	return resp, nil
}

// fetchLatestVersion resolves the latest version based on UpdateTime.
func (c *Client) fetchLatestVersion(ctx context.Context, paramName string) (*parametermanagerpb.ParameterVersion, error) {
	log.CtxLogger(ctx).Infow("Resolving latest parameter version", "parameterName", paramName)

	listResp, err := c.service.Projects.Locations.Parameters.Versions.List(paramName).Do()
	if err != nil {
		log.Logger.Errorw("Failed to list parameter versions", "parameterName", paramName, "error", err)
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}

	if len(listResp.ParameterVersions) == 0 {
		log.Logger.Warnw("No versions found for parameter", "parameterName", paramName)
		return nil, fmt.Errorf("no versions found for parameter: %s", paramName)
	}

	versions := listResp.ParameterVersions
	// Sort by UpdateTime descending to find the most recently updated version
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].UpdateTime > versions[j].UpdateTime
	})

	targetVersion := versions[0]
	log.CtxLogger(ctx).Infow("Resolved latest parameter version", "version", targetVersion.Name, "updateTime", targetVersion.UpdateTime)
	return targetVersion, nil
}

// renderVersion renders the payload for a given version.
func (c *Client) renderVersion(ctx context.Context, targetVersion *parametermanagerpb.ParameterVersion) (*Resource, error) {
	renderResp, err := c.service.Projects.Locations.Parameters.Versions.Render(targetVersion.Name).Do()
	if err != nil {
		log.Logger.Errorw("Failed to render parameter version", "versionName", targetVersion.Name, "error", err)
		return nil, fmt.Errorf("failed to render version: %w", err)
	}

	if renderResp.Payload == nil {
		log.Logger.Warnw("No payload in rendered response", "versionName", targetVersion.Name)
		return nil, fmt.Errorf("no payload in response")
	}

	data := renderResp.Payload.Data
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		log.Logger.Errorw("Failed to decode payload data", "error", err)
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}

	return &Resource{
		Data:    string(decoded),
		Version: targetVersion.Name,
	}, nil
}

// FetchParameter is a convenience function that creates a new client and fetches the parameter.
// If client is provided, it will be used. If client is nil, a new client is created for this call.
// For applications that need to fetch multiple parameters, it is more performant to create a
// single client using NewClient and reuse it for all calls.
func FetchParameter(ctx context.Context, client *Client, projectID, location, parameterName, version string) (*Resource, error) {
	if client == nil {
		newClient, err := NewClient(ctx)
		if err != nil {
			return nil, err
		}
		client = newClient
	}
	return client.FetchParameter(ctx, projectID, location, parameterName, version)
}
