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

package parametermanager

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/api/option"
	parametermanagerpb "google.golang.org/api/parametermanager/v1"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"
)

type fakeParameterManagerHandler struct {
	versionsResponse *parametermanagerpb.ListParameterVersionsResponse
	renderPayloads   map[string]string // version name -> raw string (will be base64 encoded by handler if needed)
	errorCode        int
	renderNilPayload bool
	renderBadBase64  bool
}

func (h *fakeParameterManagerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// List Versions: GET /versions?
	if strings.Contains(r.URL.Path, "/versions") && !strings.Contains(r.URL.Path, ":render") && !strings.Contains(r.URL.Path, "/versions/") {
		if h.errorCode != 0 {
			http.Error(w, "error", h.errorCode)
			return
		}
		if h.versionsResponse != nil {
			json.NewEncoder(w).Encode(h.versionsResponse)
			return
		}
	}

	// Get specific version: GET /versions/{version}
	// Note: The List handler checks !contains /versions/ so this one matches specifics.
	if strings.Contains(r.URL.Path, "/versions/") && !strings.Contains(r.URL.Path, ":render") && r.Method == "GET" {
		if h.errorCode != 0 {
			http.Error(w, "error", h.errorCode)
			return
		}
		for _, v := range h.versionsResponse.ParameterVersions {
			if strings.HasSuffix(r.URL.Path, v.Name) {
				json.NewEncoder(w).Encode(v)
				return
			}
		}
		// If specific version not found in list, return 404
		http.Error(w, "version not found", http.StatusNotFound)
		return
	}

	// Render Version: GET /versions/{version}:render
	if strings.Contains(r.URL.Path, ":render") {
		if h.errorCode != 0 {
			http.Error(w, "error", h.errorCode)
			return
		}
		parts := strings.Split(strings.TrimSuffix(r.URL.Path, ":render"), "/")
		versionName := parts[len(parts)-1]

		val, ok := h.renderPayloads[versionName]

		if ok {
			if h.renderNilPayload {
				resp := &parametermanagerpb.RenderParameterVersionResponse{
					Payload:          nil,
					ParameterVersion: versionName,
				}
				json.NewEncoder(w).Encode(resp)
				return
			}
			encoded := base64.StdEncoding.EncodeToString([]byte(val))
			if h.renderBadBase64 {
				encoded = "bad-base64-string%%"
			}
			resp := &parametermanagerpb.RenderParameterVersionResponse{
				Payload: &parametermanagerpb.ParameterVersionPayload{
					Data: encoded,
				},
				ParameterVersion: versionName,
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		http.Error(w, "render not found", http.StatusNotFound)
		return
	}

	http.Error(w, "Unknown request", http.StatusNotFound)
}

func setupTestServer(ctx context.Context, t *testing.T, handler http.Handler) (*Client, func()) {
	t.Helper()
	server := httptest.NewServer(handler)

	// Initialize logger for test
	log.SetupLoggingForTest()

	client, err := NewClient(ctx, option.WithoutAuthentication(), option.WithEndpoint(server.URL))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	return client, server.Close
}

func TestFetchParameter(t *testing.T) {
	ctx := context.Background()
	project := "test-project"
	location := "global"
	paramName := "test-param"

	// Time references (RFC3339)
	t1 := "2024-01-01T10:00:00Z"
	t2 := "2024-01-02T10:00:00Z" // Newer

	fullParamName := fmt.Sprintf("projects/%s/locations/%s/parameters/%s", project, location, paramName)

	tests := []struct {
		name        string
		version     string
		handler     *fakeParameterManagerHandler
		wantData    string
		wantVersion string
		wantErr     bool
	}{
		{
			name:    "SuccessLatestVersion",
			version: "",
			handler: &fakeParameterManagerHandler{
				versionsResponse: &parametermanagerpb.ListParameterVersionsResponse{
					ParameterVersions: []*parametermanagerpb.ParameterVersion{
						{Name: fullParamName + "/versions/v1", UpdateTime: t1, CreateTime: t1},
						{Name: fullParamName + "/versions/v2", UpdateTime: t2, CreateTime: t2},
					},
				},
				renderPayloads: map[string]string{
					"v2": "payload-v2",
				},
			},
			wantData:    "payload-v2",
			wantVersion: fullParamName + "/versions/v2",
		},
		{
			name:    "SuccessSpecificVersion",
			version: "v1",
			handler: &fakeParameterManagerHandler{
				versionsResponse: &parametermanagerpb.ListParameterVersionsResponse{
					ParameterVersions: []*parametermanagerpb.ParameterVersion{
						{Name: fullParamName + "/versions/v1", UpdateTime: t1, CreateTime: t1},
					},
				},
				renderPayloads: map[string]string{
					"v1": "payload-v1",
				},
			},
			wantData:    "payload-v1",
			wantVersion: fullParamName + "/versions/v1",
		},
		{
			name:    "ListFailure",
			version: "",
			handler: &fakeParameterManagerHandler{
				errorCode: http.StatusInternalServerError,
			},
			wantErr: true,
		},
		{
			name:    "GetVersionFailure",
			version: "v1",
			handler: &fakeParameterManagerHandler{
				errorCode: http.StatusInternalServerError,
			},
			wantErr: true,
		},
		{
			name:    "NoVersionsFound",
			version: "",
			handler: &fakeParameterManagerHandler{
				versionsResponse: &parametermanagerpb.ListParameterVersionsResponse{
					ParameterVersions: []*parametermanagerpb.ParameterVersion{},
				},
			},
			wantErr: true,
		},
		{
			name:    "RenderNilPayload",
			version: "v1",
			handler: &fakeParameterManagerHandler{
				versionsResponse: &parametermanagerpb.ListParameterVersionsResponse{
					ParameterVersions: []*parametermanagerpb.ParameterVersion{
						{Name: fullParamName + "/versions/v1", UpdateTime: t1},
					},
				},
				renderPayloads: map[string]string{
					"v1": "payload-v1",
				},
				renderNilPayload: true,
			},
			wantErr: true,
		},
		{
			name:    "RenderBadBase64",
			version: "v1",
			handler: &fakeParameterManagerHandler{
				versionsResponse: &parametermanagerpb.ListParameterVersionsResponse{
					ParameterVersions: []*parametermanagerpb.ParameterVersion{
						{Name: fullParamName + "/versions/v1", UpdateTime: t1},
					},
				},
				renderPayloads: map[string]string{
					"v1": "payload-v1",
				},
				renderBadBase64: true,
			},
			wantErr: true,
		},
		{
			name:    "RenderFailure",
			version: "",
			handler: &fakeParameterManagerHandler{
				versionsResponse: &parametermanagerpb.ListParameterVersionsResponse{
					ParameterVersions: []*parametermanagerpb.ParameterVersion{
						{Name: fullParamName + "/versions/v1", UpdateTime: t1},
					},
				},
				renderPayloads: map[string]string{}, // Missing payload
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, teardown := setupTestServer(ctx, t, tc.handler)
			defer teardown()

			got, err := client.FetchParameter(ctx, project, location, paramName, tc.version)

			if (err != nil) != tc.wantErr {
				t.Fatalf("FetchParameter(%q, %q, %q, %q) error = %v, wantErr %v", project, location, paramName, tc.version, err, tc.wantErr)
			}
			if err != nil {
				return
			}

			if got.Data != tc.wantData {
				t.Errorf("FetchParameter(%s, %s, %s, %s) data = %q, want %q", project, location, paramName, tc.version, got.Data, tc.wantData)
			}
			if got.Version != tc.wantVersion {
				t.Errorf("FetchParameter(%s, %s, %s, %s) version = %q, want %q", project, location, paramName, tc.version, got.Version, tc.wantVersion)
			}
		})
	}
}

func TestFetchParameterConvenience(t *testing.T) {
	ctx := context.Background()
	project := "test-project"
	location := "global"
	paramName := "test-param"
	t1 := "2024-01-01T10:00:00Z"
	fullParamName := fmt.Sprintf("projects/%s/locations/%s/parameters/%s", project, location, paramName)

	handler := &fakeParameterManagerHandler{
		versionsResponse: &parametermanagerpb.ListParameterVersionsResponse{
			ParameterVersions: []*parametermanagerpb.ParameterVersion{
				{Name: fullParamName + "/versions/v1", UpdateTime: t1, CreateTime: t1},
			},
		},
		renderPayloads: map[string]string{
			"v1": "payload-v1",
		},
	}

	t.Run("with client", func(t *testing.T) {
		client, teardown := setupTestServer(ctx, t, handler)
		defer teardown()

		got, err := FetchParameter(ctx, client, project, location, paramName, "v1")
		if err != nil {
			t.Fatalf("FetchParameter(with client) returned unexpected error: %v", err)
		}
		if got.Data != "payload-v1" {
			t.Errorf("FetchParameter(with client) data = %q, want %q", got.Data, "payload-v1")
		}
		if got.Version != fullParamName+"/versions/v1" {
			t.Errorf("FetchParameter(with client) version = %q, want %q", got.Version, fullParamName+"/versions/v1")
		}
	})

}
