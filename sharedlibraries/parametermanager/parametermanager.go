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
	"time"

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
	if projectID == "" || location == "" || parameterName == "" {
		return nil, fmt.Errorf("projectID, location, and parameterName must not be empty")
	}

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

	resp, err := c.service.Projects.Locations.Parameters.Versions.Get(versionName).Context(ctx).Do()
	if err != nil {
		log.CtxLogger(ctx).Errorw("Failed to get parameter version", "versionName", versionName, "error", err)
		return nil, fmt.Errorf("failed to get parameter version: %w", err)
	}
	return resp, nil
}

// parseVersionTime parses the UpdateTime (or falls back to CreateTime) of a ParameterVersion.
func parseVersionTime(v *parametermanagerpb.ParameterVersion) time.Time {
	if v == nil {
		return time.Time{}
	}
	if v.UpdateTime != "" {
		if t, err := time.Parse(time.RFC3339Nano, v.UpdateTime); err == nil {
			return t
		}
	}
	if v.CreateTime != "" {
		if t, err := time.Parse(time.RFC3339Nano, v.CreateTime); err == nil {
			return t
		}
	}
	return time.Time{}
}

// fetchLatestVersion resolves the latest version based on UpdateTime across all pages.
func (c *Client) fetchLatestVersion(ctx context.Context, paramName string) (*parametermanagerpb.ParameterVersion, error) {
	log.CtxLogger(ctx).Infow("Resolving latest parameter version", "parameterName", paramName)

	var activeVersions []*parametermanagerpb.ParameterVersion
	pageToken := ""
	for {
		req := c.service.Projects.Locations.Parameters.Versions.List(paramName).Context(ctx)
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}
		listResp, err := req.Do()
		if err != nil {
			log.CtxLogger(ctx).Errorw("Failed to list parameter versions", "parameterName", paramName, "error", err)
			return nil, fmt.Errorf("failed to list versions: %w", err)
		}
		for _, v := range listResp.ParameterVersions {
			if !v.Disabled {
				activeVersions = append(activeVersions, v)
			}
		}
		pageToken = listResp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	if len(activeVersions) == 0 {
		log.CtxLogger(ctx).Warnw("No active versions found for parameter", "parameterName", paramName)
		return nil, fmt.Errorf("no active versions found for parameter: %s", paramName)
	}

	// Sort by UpdateTime/CreateTime descending to find the most recently updated version
	sort.Slice(activeVersions, func(i, j int) bool {
		ti := parseVersionTime(activeVersions[i])
		tj := parseVersionTime(activeVersions[j])
		return ti.After(tj)
	})

	targetVersion := activeVersions[0]
	log.CtxLogger(ctx).Infow("Resolved latest parameter version", "version", targetVersion.Name, "updateTime", targetVersion.UpdateTime)
	return targetVersion, nil
}

// renderVersion renders the payload for a given version.
func (c *Client) renderVersion(ctx context.Context, targetVersion *parametermanagerpb.ParameterVersion) (*Resource, error) {
	if targetVersion == nil || targetVersion.Name == "" {
		return nil, fmt.Errorf("targetVersion and targetVersion.Name must not be empty")
	}
	renderResp, err := c.service.Projects.Locations.Parameters.Versions.Render(targetVersion.Name).Context(ctx).Do()
	if err != nil {
		log.CtxLogger(ctx).Errorw("Failed to render parameter version", "versionName", targetVersion.Name, "error", err)
		return nil, fmt.Errorf("failed to render version: %w", err)
	}

	if renderResp.Payload == nil {
		log.CtxLogger(ctx).Warnw("No payload in rendered response", "versionName", targetVersion.Name)
		return nil, fmt.Errorf("no payload in response")
	}

	data := renderResp.Payload.Data
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		log.CtxLogger(ctx).Errorw("Failed to decode payload data", "error", err)
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
