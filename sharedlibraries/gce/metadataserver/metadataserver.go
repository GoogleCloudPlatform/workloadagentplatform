/*
Copyright 2022 Google LLC

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

// Package metadataserver performs requests to the metadata server of a GCE instance.
//
// Interfacing with the metadata server is necessary to obtain project-level and per-instance
// metadata for use by the gcagent. Requests to the metadata server will also be used as a
// logging mechanism for gcagent usage metrics.
package metadataserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"
)

// Default values if information cannot be obtained from the metadata server.
const (
	ImageUnknown       = "unknown"
	MachineTypeUnknown = "unknown"
)

// GetCloudProperties abstracts metadataserver.FetchCloudProperties function for testability.
type GetCloudProperties func() *CloudProperties

var (
	zonePattern        = regexp.MustCompile("zones/([^/]*)")
	machineTypePattern = regexp.MustCompile("machineTypes/([^/]*)")

	// not a const so we can override in test suite.
	metadataServerURL                     = "http://metadata.google.internal/computeMetadata/v1"
	metadataNoUpcomingMaintenanceResponse = `{ "error": "no notifications have been received yet, try again later" }`
)

const (
	cloudPropertiesURI     = "/"
	maintenanceEventURI    = "/instance/maintenance-event"
	upcomingMaintenanceURI = "/instance/upcoming-maintenance"
	diskType               = "/instance/disks/"
	instanceAttribute      = "/instance/attributes/"
	universeDomainURI      = "/universe/universe-domain"

	helpString = `For information on permissions needed to access metadata refer: https://cloud.google.com/compute/docs/metadata/querying-metadata#permissions. Restart the agent after adding necessary permissions.`

	// PlatformCloudRun identifies Google Cloud Run environments.
	PlatformCloudRun = "CLOUD_RUN"

	// DefaultUniverseDomain is the universe domain of the Google Default Universe (GDU).
	DefaultUniverseDomain = "googleapis.com"

	// UniverseDomainEnvVar is the environment variable honored by Google Cloud client libraries to
	// select the universe domain used to construct API endpoints.
	UniverseDomainEnvVar = "GOOGLE_CLOUD_UNIVERSE_DOMAIN"
)

// ErrNotFound is returned (wrapped) when the metadata server responds with HTTP 404 Not Found.
var ErrNotFound = errors.New("metadata server endpoint not found")

type (
	metadataServerResponse struct {
		Project  projectInfo  `json:"project"`
		Instance instanceInfo `json:"instance"`
	}

	projectInfo struct {
		ProjectID        string `json:"projectId"`
		NumericProjectID int64  `json:"numericProjectId"`
	}

	instanceInfo struct {
		ID              int64           `json:"id"`
		Zone            string          `json:"zone"`
		Name            string          `json:"name"`
		Image           string          `json:"image"`
		MachineType     string          `json:"machineType"`
		ServiceAccounts serviceAccounts `json:"serviceAccounts"`
	}

	serviceAccounts struct {
		DefaultInfo defaultInfo `json:"default"`
	}

	defaultInfo struct {
		Scopes []string `json:"scopes"`
		Email  string   `json:"email"`
	}

	// CloudProperties contains the cloud properties of the instance.
	CloudProperties struct {
		ProjectID, NumericProjectID, InstanceID, Zone, InstanceName, Image, MachineType, Region string
		// Platform identifies the compute environment, e.g., default = GCE can be CLOUD_RUN.
		Platform            string
		JobName             string // Cloud Run job name
		Scopes              []string
		ServiceAccountEmail string
	}
)

// ReadCloudPropertiesWithRetry fetches information from the GCE metadata server with a retry mechanism.
//
// If there are any persistent errors in fetching this information, then the error will be logged
// and the return value will be nil.
func ReadCloudPropertiesWithRetry(bo backoff.BackOff) *CloudProperties {
	var (
		attempt = 1
		cp      *CloudProperties
	)
	err := backoff.Retry(func() error {
		var err error
		cp, err = requestProperties()
		if err != nil {
			log.Logger.Warnw("Error in requestCloudProperties", "attempt", attempt, "error", err)
			attempt++
		}
		return err
	}, bo)
	if err != nil {
		log.Logger.Errorw("CloudProperties request retry limit exceeded", log.Error(err))
	}
	return cp
}

// DiskTypeWithRetry fetches disk information from the GCE metadata server with a retry mechanism.
//
// If there are any persistent errors in fetching this information, then the error will be logged
// and the return value will be "".
func DiskTypeWithRetry(bo backoff.BackOff, disk string) string {
	var (
		attempt  = 1
		diskType string
	)
	err := backoff.Retry(func() error {
		var err error
		diskType, err = requestDiskType(disk)
		if err != nil {
			log.Logger.Warnw("Error in requestDiskType", "attempt", attempt, "error", err)
			attempt++
		}
		return err
	}, bo)
	if err != nil {
		log.Logger.Errorw("DiskType request retry limit exceeded", log.Error(err))
	}
	return diskType
}

// InstanceAttributeWithRetry fetches instance attributes from the GCE metadata server with a retry
// mechanism.
//
// If there are any persistent errors in fetching this information, then the error will be logged
// and the return value will be "".
func InstanceAttributeWithRetry(bo backoff.BackOff, key string) string {
	var (
		attempt = 1
		value   string
	)
	err := backoff.Retry(func() error {
		var err error
		value, err = requestInstanceAttribute(key)
		if err != nil {
			log.Logger.Warnw("Error in FetchInstanceAttributes", "attempt", attempt, "error", err)
			attempt++
		}
		return err
	}, bo)
	if err != nil {
		log.Logger.Errorw("InstanceAttributes request retry limit exceeded", log.Error(err))
	}
	return value
}

// UniverseDomainWithRetry fetches the universe domain from the GCE metadata server with a retry
// mechanism.
//
// A 404 response from the metadata server is not retried and results in DefaultUniverseDomain.
func UniverseDomainWithRetry(bo backoff.BackOff) string {
	var (
		attempt = 1
		domain  string
	)
	err := backoff.Retry(func() error {
		var err error
		domain, err = requestUniverseDomain()
		if err != nil {
			log.Logger.Warnw("Error in requestUniverseDomain", "attempt", attempt, "error", err)
			attempt++
		}
		return err
	}, bo)
	if err != nil {
		log.Logger.Warnw("UniverseDomain request retry limit exceeded, using default universe domain", "universeDomain", DefaultUniverseDomain, log.Error(err))
		return DefaultUniverseDomain
	}
	return domain
}

// get performs a get request to the metadata server and returns the response body.
func get(uri, queryString string) ([]byte, error) {
	metadataURL, err := url.Parse(metadataServerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metadata server url: %v, %s", err, helpString)
	}
	metadataURL.RawQuery = queryString
	reqURL := metadataURL.JoinPath(uri).String()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to metadata server: %v, %s", err, helpString)
	}
	req.Header.Add("Metadata-Flavor", "Google")
	client := &http.Client{Timeout: 2 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to receive response from metadata server: %v, %s", err, helpString)
	}
	defer res.Body.Close()
	if !isStatusSuccess(res.StatusCode) {
		if uri == upcomingMaintenanceURI && res.StatusCode == 503 {
			body, errIO := io.ReadAll(res.Body)
			if errIO != nil {
				return nil, fmt.Errorf("failed to read response body from metadata server: %v", err)
			}
			return body, nil
		}
		if res.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("%w: unsuccessful response from metadata server: %s, %s", ErrNotFound, res.Status, helpString)
		}
		return nil, fmt.Errorf("unsuccessful response from metadata server: %s, %s", res.Status, helpString)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from metadata server: %v", err)
	}
	return body, nil
}

// requestProperties attempts to fetch information from the GCE metadata server.
func requestProperties() (*CloudProperties, error) {
	body, err := get(cloudPropertiesURI, "recursive=true")
	if err != nil {
		return nil, fmt.Errorf("failed to get cloud properties from metadata server: %v", err)
	}
	resBodyJSON := &metadataServerResponse{}
	if err = json.Unmarshal(body, resBodyJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body from metadata server: %v", err)
	}

	project := resBodyJSON.Project
	projectID := project.ProjectID
	numericProjectID := strconv.FormatInt(int64(project.NumericProjectID), 10)
	instance := resBodyJSON.Instance
	instanceID := strconv.FormatInt(int64(instance.ID), 10)
	zone := parseZone(instance.Zone)
	machineType := parseMachineType(instance.MachineType)
	instanceName := instance.Name
	image := instance.Image
	scopes := instance.ServiceAccounts.DefaultInfo.Scopes
	serviceAccountEmail := instance.ServiceAccounts.DefaultInfo.Email

	if image == "" {
		image = ImageUnknown
	}
	if machineType == "" {
		machineType = MachineTypeUnknown
	}

	log.Logger.Debugw("Default Cloud Properties from metadata server",
		"projectid", projectID, "projectnumber", numericProjectID, "instanceid", instanceID, "zone", zone,
		"instancename", instanceName, "image", image, "machinetype", machineType, "scopes", scopes,
		"defaultserviceaccountemail", serviceAccountEmail)

	if projectID == "" || numericProjectID == "0" || instanceID == "0" || zone == "" || instanceName == "" {
		return nil, fmt.Errorf("metadata server responded with incomplete information")
	}
	region := regionFromZone(zone)

	return &CloudProperties{
		ProjectID:           projectID,
		NumericProjectID:    numericProjectID,
		InstanceID:          instanceID,
		Zone:                zone,
		Region:              region,
		InstanceName:        instanceName,
		Image:               image,
		MachineType:         machineType,
		Scopes:              scopes,
		ServiceAccountEmail: serviceAccountEmail,
	}, nil
}

// requestDiskType attempts to fetch the disk type from the GCE metadata server.
func requestDiskType(disk string) (string, error) {
	body, err := get(fmt.Sprintf("%s%s/type", diskType, disk), "recursive=true")
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// requestInstanceAttribute attempts to fetch an instance attribute from the GCE metadata server.
func requestInstanceAttribute(key string) (string, error) {
	body, err := get(fmt.Sprintf("%s%s", instanceAttribute, key), "recursive=true")
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// requestUniverseDomain attempts to fetch the universe domain from the GCE metadata server.
//
// The universe endpoint is only published in Trusted Partner Cloud (TPC) universes.
func requestUniverseDomain() (string, error) {
	body, err := get(universeDomainURI, "")
	if errors.Is(err, ErrNotFound) {
		return DefaultUniverseDomain, nil
	}
	if err != nil {
		return "", err
	}
	domain := strings.TrimSpace(string(body))
	if domain == "" {
		return DefaultUniverseDomain, nil
	}
	return domain, nil
}

func isStatusSuccess(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode <= 299
}

// parseZone retrieves the zone name from the metadata server response.
//
// The metadata server returns the zone as "projects/PROJECT_NUM/zones/ZONE_NAME" but we only need ZONE_NAME.
func parseZone(raw string) string {
	var zone string
	match := zonePattern.FindStringSubmatch(raw)
	if len(match) >= 2 {
		zone = match[1]
	}
	return zone
}

func regionFromZone(zone string) string {
	regionParts := strings.Split(zone, "-")
	if len(regionParts) < 2 {
		return ""
	}
	return strings.Join(regionParts[:2], "-")
}

// parseMachineType retrieves the machine type from the response.
// The metadata server returns the machine type as
// "projects/PROJECT_NUM/machineTypes/MACHINE_TYPE", we only need MACHINE_TYPE.
func parseMachineType(raw string) string {
	match := machineTypePattern.FindStringSubmatch(raw)
	if len(match) >= 2 {
		return match[1]
	}
	return ""
}

// FetchCloudProperties retrieves the cloud properties using a backoff policy.
func FetchCloudProperties() *CloudProperties {
	exp := backoff.NewExponentialBackOff()
	return ReadCloudPropertiesWithRetry(backoff.WithMaxRetries(exp, 1)) // 1 retry (2 total attempts)
}

// FetchGCEMaintenanceEvent retrieves information about pending host maintenance events.
func FetchGCEMaintenanceEvent() (string, error) {
	body, err := get(maintenanceEventURI, "")
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// FetchGCEUpcomingMaintenance retrieves information about upcoming host maintenance events.
func FetchGCEUpcomingMaintenance() (string, error) {
	body, err := get(upcomingMaintenanceURI, "")
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// ConfigureUniverseDomainWithRetry determines the universe domain for the current process and
// ensures it is exported via the GOOGLE_CLOUD_UNIVERSE_DOMAIN environment variable, which Google
// Cloud client libraries use to construct API endpoints. Child processes inherit the variable.
//
// The universe domain is resolved in the following order:
//  1. The GOOGLE_CLOUD_UNIVERSE_DOMAIN environment variable, if already set.
//  2. The metadata server universe/universe-domain endpoint (published in TPC universes).
//  3. DefaultUniverseDomain if the endpoint is not found (googleapis.com).
//
// The environment variable is only set when a non-default universe domain is detected, so the
// behavior in the Google Default Universe is unchanged. Returns the resolved universe domain.
func ConfigureUniverseDomainWithRetry(bo backoff.BackOff) string {
	if domain := strings.TrimSpace(os.Getenv(UniverseDomainEnvVar)); domain != "" {
		log.Logger.Debugw("Using universe domain from environment", "envVar", UniverseDomainEnvVar, "universeDomain", domain)
		return domain
	}
	domain := UniverseDomainWithRetry(bo)
	if domain == DefaultUniverseDomain {
		log.Logger.Debugw("Using default universe domain", "universeDomain", domain)
		return domain
	}
	if err := os.Setenv(UniverseDomainEnvVar, domain); err != nil {
		log.Logger.Warnw("Could not set universe domain environment variable", "envVar", UniverseDomainEnvVar, "universeDomain", domain, log.Error(err))
		return domain
	}
	log.Logger.Debugw("Detected universe domain from metadata server", "envVar", UniverseDomainEnvVar, "universeDomain", domain)
	return domain
}

// ConfigureUniverseDomain configures the universe domain using a default backoff policy.
func ConfigureUniverseDomain() string {
	exp := backoff.NewExponentialBackOff()
	return ConfigureUniverseDomainWithRetry(backoff.WithMaxRetries(exp, 1))
}

// UniverseDomain returns the universe domain for the current process.
//
// It returns the value of the GOOGLE_CLOUD_UNIVERSE_DOMAIN environment variable if set, otherwise
// DefaultUniverseDomain. It does not contact the metadata server; call ConfigureUniverseDomain at
// process startup to detect the universe domain and export the environment variable.
func UniverseDomain() string {
	if domain := strings.TrimSpace(os.Getenv(UniverseDomainEnvVar)); domain != "" {
		return domain
	}
	return DefaultUniverseDomain
}

// ServiceHost returns the API host name for a Google Cloud service in the current universe.
//
// For example, ServiceHost("storage") returns "storage.googleapis.com" in the Google Default
// Universe and "storage.apis-berlin-build0.goog" in the TSP Trusted Partner Cloud universe.
func ServiceHost(service string) string {
	return fmt.Sprintf("%s.%s", service, UniverseDomain())
}

// ServiceEndpoint returns the HTTPS API endpoint (without a trailing slash) for a Google Cloud
// service in the current universe, e.g. "https://compute.googleapis.com".
func ServiceEndpoint(service string) string {
	return "https://" + ServiceHost(service)
}

// ComputeResourcePrefix returns the prefix of fully qualified Compute Engine resource URLs (as
// returned in selfLink fields) for the current universe, including the trailing slash.
//
// The Google Default Universe uses "https://www.googleapis.com/compute/v1/", while other universes
// use the compute service endpoint, e.g. "https://compute.apis-berlin-build0.goog/compute/v1/".
func ComputeResourcePrefix() string {
	if domain := UniverseDomain(); domain != DefaultUniverseDomain {
		return ServiceEndpoint("compute") + "/compute/v1/"
	}
	return "https://www.googleapis.com/compute/v1/"
}
