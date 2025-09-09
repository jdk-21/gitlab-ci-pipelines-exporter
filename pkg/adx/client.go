package adx

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/data/errors"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	log "github.com/sirupsen/logrus"

	"github.com/mvisonneau/gitlab-ci-pipelines-exporter/pkg/config"
	"github.com/mvisonneau/gitlab-ci-pipelines-exporter/pkg/schemas"
)

// Client wraps Azure Data Explorer client functionality.
type Client struct {
	config      config.AzureDataExplorer
	kustoClient *kusto.Client
	ingestor    ingest.Ingestor
	batchChan   chan MetricData
	closeChan   chan struct{}
}

// MetricData represents a metric to be sent to Azure Data Explorer.
type MetricData struct {
	Timestamp   time.Time         `json:"timestamp"`
	MetricName  string            `json:"metric_name"`
	MetricValue float64           `json:"metric_value"`
	Labels      map[string]string `json:"labels"`
	Project     string            `json:"project"`
	Ref         string            `json:"ref"`
	Environment string            `json:"environment,omitempty"`
}

// getMetricName returns the prometheus metric name for a given MetricKind.
func getMetricName(kind schemas.MetricKind) string {
	switch kind {
	case schemas.MetricKindCoverage:
		return "gitlab_ci_pipeline_coverage"
	case schemas.MetricKindDurationSeconds:
		return "gitlab_ci_pipeline_duration_seconds"
	case schemas.MetricKindEnvironmentBehindCommitsCount:
		return "gitlab_ci_environment_behind_commits_count"
	case schemas.MetricKindEnvironmentBehindDurationSeconds:
		return "gitlab_ci_environment_behind_duration_seconds"
	case schemas.MetricKindEnvironmentDeploymentCount:
		return "gitlab_ci_environment_deployment_count"
	case schemas.MetricKindEnvironmentDeploymentDurationSeconds:
		return "gitlab_ci_environment_deployment_duration_seconds"
	case schemas.MetricKindEnvironmentDeploymentJobID:
		return "gitlab_ci_environment_deployment_job_id"
	case schemas.MetricKindEnvironmentDeploymentStatus:
		return "gitlab_ci_environment_deployment_status"
	case schemas.MetricKindEnvironmentDeploymentTimestamp:
		return "gitlab_ci_environment_deployment_timestamp"
	case schemas.MetricKindEnvironmentInformation:
		return "gitlab_ci_environment_information"
	case schemas.MetricKindID:
		return "gitlab_ci_pipeline_id"
	case schemas.MetricKindJobArtifactSizeBytes:
		return "gitlab_ci_job_artifact_size_bytes"
	case schemas.MetricKindJobDurationSeconds:
		return "gitlab_ci_job_duration_seconds"
	case schemas.MetricKindJobID:
		return "gitlab_ci_job_id"
	case schemas.MetricKindJobQueuedDurationSeconds:
		return "gitlab_ci_job_queued_duration_seconds"
	case schemas.MetricKindJobRunCount:
		return "gitlab_ci_job_run_count"
	case schemas.MetricKindJobStatus:
		return "gitlab_ci_job_status"
	case schemas.MetricKindJobTimestamp:
		return "gitlab_ci_job_timestamp"
	case schemas.MetricKindQueuedDurationSeconds:
		return "gitlab_ci_pipeline_queued_duration_seconds"
	case schemas.MetricKindRunCount:
		return "gitlab_ci_pipeline_run_count"
	case schemas.MetricKindStatus:
		return "gitlab_ci_pipeline_status"
	case schemas.MetricKindTimestamp:
		return "gitlab_ci_pipeline_timestamp"
	case schemas.MetricKindTestReportTotalCount:
		return "gitlab_ci_pipeline_test_report_total_count"
	case schemas.MetricKindTestReportSuccessCount:
		return "gitlab_ci_pipeline_test_report_success_count"
	case schemas.MetricKindTestReportFailedCount:
		return "gitlab_ci_pipeline_test_report_failed_count"
	case schemas.MetricKindTestReportSkippedCount:
		return "gitlab_ci_pipeline_test_report_skipped_count"
	case schemas.MetricKindTestReportErrorCount:
		return "gitlab_ci_pipeline_test_report_error_count"
	case schemas.MetricKindTestReportTotalTime:
		return "gitlab_ci_pipeline_test_report_total_time"
	case schemas.MetricKindTestSuiteTotalCount:
		return "gitlab_ci_pipeline_test_suite_total_count"
	case schemas.MetricKindTestSuiteSuccessCount:
		return "gitlab_ci_pipeline_test_suite_success_count"
	case schemas.MetricKindTestSuiteFailedCount:
		return "gitlab_ci_pipeline_test_suite_failed_count"
	case schemas.MetricKindTestSuiteSkippedCount:
		return "gitlab_ci_pipeline_test_suite_skipped_count"
	case schemas.MetricKindTestSuiteErrorCount:
		return "gitlab_ci_pipeline_test_suite_error_count"
	case schemas.MetricKindTestCaseExecutionTime:
		return "gitlab_ci_pipeline_test_case_execution_time"
	case schemas.MetricKindTestCaseStatus:
		return "gitlab_ci_pipeline_test_case_status"
	default:
		return fmt.Sprintf("gitlab_ci_metric_%d", int(kind))
	}
}

// NewClient creates a new Azure Data Explorer client.
func NewClient(ctx context.Context, cfg config.AzureDataExplorer) (*Client, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	log.WithFields(log.Fields{
		"cluster_url": cfg.ClusterURL,
		"database":    cfg.Database,
		"table":       cfg.Table,
		"auth_method": cfg.AuthMethod,
	}).Info("initializing Azure Data Explorer client")

	// Create authentication provider based on configuration
	var kcsb *kusto.ConnectionStringBuilder
	var err error

	switch cfg.AuthMethod {
	case "managed_identity":
		if cfg.ClientID != "" {
			kcsb = kusto.NewConnectionStringBuilder(cfg.ClusterURL).WithUserManagedIdentity(cfg.ClientID)
		} else {
			kcsb = kusto.NewConnectionStringBuilder(cfg.ClusterURL).WithSystemManagedIdentity()
		}
	case "client_credentials":
		if cfg.ClientSecret == "" {
			return nil, fmt.Errorf("client_secret is required for client_credentials auth method")
		}
		kcsb = kusto.NewConnectionStringBuilder(cfg.ClusterURL).WithAadAppKey(cfg.ClientID, cfg.ClientSecret, cfg.TenantID)
	case "device_code":
		kcsb = kusto.NewConnectionStringBuilder(cfg.ClusterURL).WithInteractiveLogin(cfg.TenantID)
	default:
		return nil, fmt.Errorf("unsupported auth method: %s", cfg.AuthMethod)
	}

	// Create Kusto client
	kustoClient, err := kusto.New(kcsb)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kusto client: %w", err)
	}

	// Create ingestor for data ingestion
	ingestor, err := ingest.New(kustoClient, cfg.Database, cfg.Table)
	if err != nil {
		return nil, fmt.Errorf("failed to create ingestor: %w", err)
	}

	client := &Client{
		config:      cfg,
		kustoClient: kustoClient,
		ingestor:    ingestor,
		batchChan:   make(chan MetricData, cfg.BatchSize*2), // Buffer twice the batch size
		closeChan:   make(chan struct{}),
	}

	// Start the batch processor
	go client.processBatches(ctx)

	log.Info("Azure Data Explorer client initialized successfully")
	return client, nil
}

// SendMetric sends a single metric to Azure Data Explorer (via batching).
func (c *Client) SendMetric(ctx context.Context, metric schemas.Metric) error {
	if c == nil {
		return nil // ADX not enabled
	}

	metricData := MetricData{
		Timestamp:   time.Now(),
		MetricName:  getMetricName(metric.Kind),
		MetricValue: metric.Value,
		Labels:      metric.Labels,
		Project:     metric.Labels["project"],
		Ref:         metric.Labels["ref"],
	}

	// Add environment if present
	if env, exists := metric.Labels["environment"]; exists {
		metricData.Environment = env
	}

	select {
	case c.batchChan <- metricData:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		log.Warn("ADX batch channel is full, dropping metric")
		return fmt.Errorf("batch channel is full")
	}
}

// SendMetrics sends multiple metrics to Azure Data Explorer.
func (c *Client) SendMetrics(ctx context.Context, metrics []schemas.Metric) error {
	if c == nil {
		return nil // ADX not enabled
	}

	for _, metric := range metrics {
		if err := c.SendMetric(ctx, metric); err != nil {
			log.WithError(err).Warn("failed to queue metric for ADX")
		}
	}
	return nil
}

// processBatches processes metrics in batches and sends them to ADX.
func (c *Client) processBatches(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(c.config.FlushIntervalSeconds) * time.Second)
	defer ticker.Stop()

	batch := make([]MetricData, 0, c.config.BatchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}

		if err := c.ingestBatch(ctx, batch); err != nil {
			log.WithError(err).Error("failed to ingest batch to ADX")
		} else {
			log.WithField("batch_size", len(batch)).Debug("successfully ingested batch to ADX")
		}
		batch = batch[:0] // Reset batch
	}

	for {
		select {
		case metric := <-c.batchChan:
			batch = append(batch, metric)
			if len(batch) >= c.config.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-c.closeChan:
			flush() // Flush remaining metrics before closing
			return
		case <-ctx.Done():
			flush() // Flush remaining metrics before closing
			return
		}
	}
}

// ingestBatch ingests a batch of metrics to Azure Data Explorer.
func (c *Client) ingestBatch(ctx context.Context, batch []MetricData) error {
	if len(batch) == 0 {
		return nil
	}

	// Convert batch to JSON
	jsonData, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("failed to marshal batch to JSON: %w", err)
	}

	// Create ingestion from JSON data
	from := strings.NewReader(string(jsonData))
	_, err = c.ingestor.FromReader(ctx, from, ingest.FileFormat(ingest.JSON), ingest.IgnoreFirstRecord())
	if err != nil {
		if kustoErr, ok := errors.GetKustoError(err); ok {
			return fmt.Errorf("kusto ingestion error: %s", kustoErr.Error())
		}
		return fmt.Errorf("failed to ingest batch: %w", err)
	}

	return nil
}

// Close closes the Azure Data Explorer client and flushes any remaining data.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}

	log.Info("closing Azure Data Explorer client")
	close(c.closeChan)

	// Close the underlying clients
	if c.kustoClient != nil {
		c.kustoClient.Close()
	}

	log.Info("Azure Data Explorer client closed")
	return nil
}