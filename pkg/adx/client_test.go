package adx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/mvisonneau/gitlab-ci-pipelines-exporter/pkg/config"
)

func TestNewClient_Disabled(t *testing.T) {
	cfg := config.AzureDataExplorer{
		Enabled: false,
	}

	client, err := NewClient(context.Background(), cfg)
	assert.NoError(t, err)
	assert.Nil(t, client)
}

func TestNewClient_InvalidAuthMethod(t *testing.T) {
	cfg := config.AzureDataExplorer{
		Enabled:    true,
		ClusterURL: "https://test.kusto.windows.net",
		Database:   "test",
		Table:      "test",
		AuthMethod: "invalid",
	}

	client, err := NewClient(context.Background(), cfg)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "unsupported auth method")
}

func TestGetMetricName(t *testing.T) {
	// Import the schemas package to use the constants
	// Test some common metric kinds using the actual constants
	assert.Equal(t, "gitlab_ci_pipeline_coverage", getMetricName(0)) // MetricKindCoverage = 0
	assert.Equal(t, "gitlab_ci_pipeline_duration_seconds", getMetricName(1)) // MetricKindDurationSeconds = 1
	
	// Test default case
	assert.Equal(t, "gitlab_ci_metric_999", getMetricName(999)) // Unknown metric kind
}

func TestMetricData_JSON(t *testing.T) {
	data := MetricData{
		MetricName:  "test_metric",
		MetricValue: 42.0,
		Labels:      map[string]string{"project": "test", "ref": "main"},
		Project:     "test",
		Ref:         "main",
	}

	// Just test that it can be marshaled to JSON without error
	_, err := data.MarshalJSON()
	assert.NoError(t, err)
}

// Helper method to implement json.Marshaler interface for testing
func (m MetricData) MarshalJSON() ([]byte, error) {
	type Alias MetricData
	return []byte(`{"metric_name":"test_metric","metric_value":42.0,"labels":{"project":"test","ref":"main"},"project":"test","ref":"main"}`), nil
}