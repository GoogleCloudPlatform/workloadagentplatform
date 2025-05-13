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

package timeseries

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	mpb "google.golang.org/genproto/googleapis/api/metric"
	mrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	cpb "google.golang.org/genproto/googleapis/monitoring/v3"
	mrpb "google.golang.org/genproto/googleapis/monitoring/v3"
	tpb "google.golang.org/protobuf/types/known/timestamppb"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/gce/metadataserver"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"
)

func TestMain(t *testing.M) {
	log.SetupLoggingForTest()
	os.Exit(t.Run())
}

var (
	mType   = "test/metric"
	mLabels = map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	gceLabels = map[string]string{
		"project_id":  "test-project",
		"zone":        "test-zone",
		"instance_id": "123456",
	}
	bmsLabels = map[string]string{
		"resource_container": "test-project",
		"location":           "test-location",
		"instance_id":        "test-bms-id",
	}
	defaultCloudProperties = &metadataserver.CloudProperties{
		ProjectID:  "test-project",
		Zone:       "test-zone",
		InstanceID: "123456",
	}
	bmsCloudProperties = &metadataserver.CloudProperties{
		ProjectID:    "test-project",
		Region:       "test-location",
		InstanceName: "test-bms",
		InstanceID:   "test-bms-id",
	}
	now = &tpb.Timestamp{
		Seconds: 1234,
	}
)

func TestBuildInt(t *testing.T) {
	// Not using table driven tests as we do not have conditional behavior based on inputs.
	want := &mrpb.TimeSeries{
		Metric: &mpb.Metric{
			Type:   mType,
			Labels: mLabels,
		},
		MetricKind: mpb.MetricDescriptor_GAUGE,
		Resource: &mrespb.MonitoredResource{
			Type:   "gce_instance",
			Labels: gceLabels,
		},
		Points: []*mrpb.Point{{
			Interval: &cpb.TimeInterval{
				StartTime: now,
				EndTime:   now,
			},
			Value: &cpb.TypedValue{
				Value: &cpb.TypedValue_Int64Value{
					Int64Value: 100,
				},
			},
		}},
	}

	p := Params{
		CloudProp:    defaultCloudProperties,
		MetricType:   mType,
		MetricLabels: mLabels,
		Timestamp:    now,
		Int64Value:   100,
	}
	got := BuildInt(p)
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("Failure in BuildInt(), (-want +got):\n%s", diff)
	}
}

func TestBuildBool(t *testing.T) {
	// Not using table driven tests as we do not have conditional behavior based on inputs.
	want := &mrpb.TimeSeries{
		Metric: &mpb.Metric{
			Type:   mType,
			Labels: mLabels,
		},
		MetricKind: mpb.MetricDescriptor_GAUGE,
		Resource: &mrespb.MonitoredResource{
			Type:   "gce_instance",
			Labels: gceLabels,
		},
		Points: []*mrpb.Point{{
			Interval: &cpb.TimeInterval{
				StartTime: now,
				EndTime:   now,
			},
			Value: &cpb.TypedValue{
				Value: &cpb.TypedValue_BoolValue{
					BoolValue: true,
				},
			},
		}},
	}

	p := Params{
		CloudProp:    defaultCloudProperties,
		MetricType:   mType,
		MetricLabels: mLabels,
		Timestamp:    now,
		BoolValue:    true,
	}
	got := BuildBool(p)
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("Failure in BuildBool(), (-want +got):\n%s", diff)
	}
}

func TestBuildFloat64(t *testing.T) {
	want := &mrpb.TimeSeries{
		Metric: &mpb.Metric{
			Type:   mType,
			Labels: mLabels,
		},
		MetricKind: mpb.MetricDescriptor_GAUGE,
		Resource: &mrespb.MonitoredResource{
			Type:   "gce_instance",
			Labels: gceLabels,
		},
		Points: []*mrpb.Point{{
			Interval: &cpb.TimeInterval{
				StartTime: now,
				EndTime:   now,
			},
			Value: &cpb.TypedValue{
				Value: &cpb.TypedValue_DoubleValue{
					DoubleValue: 100.32,
				},
			},
		}},
	}

	p := Params{
		CloudProp:    defaultCloudProperties,
		MetricType:   mType,
		MetricLabels: mLabels,
		Timestamp:    now,
		Float64Value: 100.32,
	}
	got := BuildFloat64(p)
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("Failure in BuildFloat64, (-want +got):\n%s", diff)
	}
}

func TestMonitoredResource(t *testing.T) {
	tests := []struct {
		name           string
		cloudProps     *metadataserver.CloudProperties
		bareMetal      bool
		healthbeat     bool
		resourceLabels map[string]string
		want           *mrespb.MonitoredResource
	}{
		{
			name:       "BareMetal",
			cloudProps: bmsCloudProperties,
			bareMetal:  true,
			want: &mrespb.MonitoredResource{
				Type:   "baremetalsolution.googleapis.com/Instance",
				Labels: bmsLabels,
			},
		},
		{
			name:       "GCE",
			cloudProps: defaultCloudProperties,
			want: &mrespb.MonitoredResource{
				Type:   "gce_instance",
				Labels: gceLabels,
			},
		},

		{
			name:           "BareMetalHeartbeat",
			cloudProps:     bmsCloudProperties,
			bareMetal:      true,
			healthbeat:     true,
			resourceLabels: map[string]string{"instance_id": "123"},
			want: &mrespb.MonitoredResource{
				Type:   "compute.googleapis.com/WorkloadProcess",
				Labels: map[string]string{"instance_id": "123"},
			},
		},
		{
			name:           "GCEHeartbeat",
			cloudProps:     defaultCloudProperties,
			healthbeat:     true,
			resourceLabels: map[string]string{"instance_id": "123"},
			want: &mrespb.MonitoredResource{
				Type:   "compute.googleapis.com/WorkloadProcess",
				Labels: map[string]string{"instance_id": "123"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := monitoredResource(test.cloudProps, test.bareMetal, test.healthbeat, test.resourceLabels)
			if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("Failure in monitoredResource(), (-want +got):\n%s", diff)
			}
		})
	}
}
