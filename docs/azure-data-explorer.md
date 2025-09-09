# Azure Data Explorer Integration

This document explains how to configure and use the Azure Data Explorer (ADX) integration in the GitLab CI Pipelines Exporter to send metrics directly to Azure Data Explorer instead of only exposing them via the Prometheus endpoint.

## Overview

The Azure Data Explorer integration allows you to:
- Send GitLab CI pipeline metrics directly to Azure Data Explorer
- Use ADX's powerful query capabilities for advanced analytics
- Store historical data for long-term trend analysis
- Integrate with Azure monitoring and alerting systems

## Prerequisites

1. **Azure Data Explorer Cluster**: You need an existing ADX cluster
2. **Database and Table**: Create a database and table in your ADX cluster
3. **Authentication**: Configure appropriate authentication method
4. **Permissions**: Ensure the authentication method has ingest permissions

## Table Schema

Create a table in your Azure Data Explorer database with the following schema:

```kql
.create table ci_pipeline_metrics (
    timestamp: datetime,
    metric_name: string,
    metric_value: real,
    labels: dynamic,
    project: string,
    ref: string,
    environment: string
)
```

## Configuration

Add the `azure_data_explorer` section to your configuration file:

```yaml
azure_data_explorer:
  enabled: true
  cluster_url: "https://mycluster.westus2.kusto.windows.net"
  database: "gitlab_metrics"
  table: "ci_pipeline_metrics"
  auth_method: "managed_identity"
  batch_size: 1000
  flush_interval_seconds: 30
```

### Configuration Options

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `enabled` | boolean | No | `false` | Enable/disable ADX integration |
| `cluster_url` | string | Yes* | - | ADX cluster URL (required if enabled) |
| `database` | string | Yes* | - | Database name (required if enabled) |
| `table` | string | Yes* | - | Table name (required if enabled) |
| `auth_method` | string | No | `managed_identity` | Authentication method |
| `client_id` | string | No | - | Client ID for authentication |
| `client_secret` | string | No | - | Client secret (required for client_credentials) |
| `tenant_id` | string | No | - | Tenant ID for authentication |
| `batch_size` | integer | No | `1000` | Number of metrics to batch together |
| `flush_interval_seconds` | integer | No | `30` | Interval to flush batched metrics |

*Required only when `enabled` is `true`

### Authentication Methods

#### 1. Managed Identity (Default)
```yaml
azure_data_explorer:
  enabled: true
  cluster_url: "https://mycluster.westus2.kusto.windows.net"
  database: "gitlab_metrics"
  table: "ci_pipeline_metrics"
  auth_method: "managed_identity"
```

For user-assigned managed identity:
```yaml
azure_data_explorer:
  enabled: true
  cluster_url: "https://mycluster.westus2.kusto.windows.net"
  database: "gitlab_metrics"
  table: "ci_pipeline_metrics"
  auth_method: "managed_identity"
  client_id: "your-user-assigned-identity-client-id"
```

#### 2. Client Credentials (Service Principal)
```yaml
azure_data_explorer:
  enabled: true
  cluster_url: "https://mycluster.westus2.kusto.windows.net"
  database: "gitlab_metrics"
  table: "ci_pipeline_metrics"
  auth_method: "client_credentials"
  client_id: "your-service-principal-client-id"
  client_secret: "your-service-principal-client-secret"
  tenant_id: "your-azure-tenant-id"
```

#### 3. Device Code (Interactive)
```yaml
azure_data_explorer:
  enabled: true
  cluster_url: "https://mycluster.westus2.kusto.windows.net"
  database: "gitlab_metrics"
  table: "ci_pipeline_metrics"
  auth_method: "device_code"
  tenant_id: "your-azure-tenant-id"
```

## Data Format

Metrics are sent to ADX in the following JSON format:

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "metric_name": "gitlab_ci_pipeline_duration_seconds",
  "metric_value": 120.5,
  "labels": {
    "project": "my-org/my-project",
    "ref": "main",
    "status": "success"
  },
  "project": "my-org/my-project",
  "ref": "main",
  "environment": "production"
}
```

## Querying Data

Once data is flowing to ADX, you can query it using KQL (Kusto Query Language):

### Basic Queries

```kql
// Get all metrics for a specific project
ci_pipeline_metrics
| where project == "my-org/my-project"
| take 100

// Average pipeline duration by project
ci_pipeline_metrics
| where metric_name == "gitlab_ci_pipeline_duration_seconds"
| summarize avg_duration = avg(metric_value) by project

// Pipeline success rate over time
ci_pipeline_metrics
| where metric_name == "gitlab_ci_pipeline_status"
| extend status = tostring(labels.status)
| summarize 
    total = count(),
    successful = countif(status == "success")
    by bin(timestamp, 1h)
| extend success_rate = successful * 100.0 / total
```

### Advanced Analytics

```kql
// Trend analysis: Pipeline duration over time
ci_pipeline_metrics
| where metric_name == "gitlab_ci_pipeline_duration_seconds"
| where timestamp > ago(30d)
| summarize 
    avg_duration = avg(metric_value),
    p95_duration = percentile(metric_value, 95)
    by bin(timestamp, 1d), project
| render timechart

// Identify problematic branches
ci_pipeline_metrics
| where metric_name == "gitlab_ci_pipeline_status"
| extend status = tostring(labels.status)
| where timestamp > ago(7d)
| summarize 
    total = count(),
    failed = countif(status == "failed"),
    success_rate = (count() - countif(status == "failed")) * 100.0 / count()
    by project, ref
| where success_rate < 90
| order by success_rate asc
```

## Monitoring and Troubleshooting

### Logs

The exporter logs ADX-related activities. Look for these log messages:

- `"initializing Azure Data Explorer client"` - ADX client initialization
- `"successfully ingested batch to ADX"` - Successful data ingestion
- `"failed to ingest batch to ADX"` - Ingestion failures
- `"ADX batch channel is full, dropping metric"` - Backpressure issues

### Common Issues

1. **Authentication Failures**
   - Verify credentials and permissions
   - Check that the identity has `Database Ingestor` role on the database

2. **Connection Issues**
   - Verify cluster URL is correct
   - Check network connectivity to Azure

3. **Ingestion Failures**
   - Verify database and table names
   - Check table schema matches expected format
   - Monitor ADX cluster health and ingestion limits

### Performance Tuning

- **Batch Size**: Increase for higher throughput, decrease for lower latency
- **Flush Interval**: Reduce for more real-time data, increase for better efficiency
- **Buffer Size**: The internal channel buffer is 2x the batch size

## Integration with Existing Monitoring

The ADX integration runs alongside the existing Prometheus metrics endpoint. You can:

- Continue using Prometheus for real-time monitoring and alerting
- Use ADX for historical analysis and advanced analytics
- Gradually migrate monitoring workflows to ADX
- Use both systems in parallel for different use cases

## Example Complete Configuration

See [examples/azure-data-explorer.yml](../examples/azure-data-explorer.yml) for a complete configuration example.