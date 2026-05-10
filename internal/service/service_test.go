package service

import (
	"context"
	"path/filepath"
	"testing"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"

	"github.com/kamikaze011001/claude-code-observer/internal/repository"
)

func openTempRepo(t *testing.T) *repository.Repository {
	t.Helper()
	repo, err := repository.Open(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func kvStr(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}

func TestService_IngestLogs_WritesAllRecords(t *testing.T) {
	repo := openTempRepo(t)
	svc := New(repo, nil)

	req := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvStr("project.name", "demo")}},
			ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{
				{TimeUnixNano: 1, Attributes: []*commonpb.KeyValue{
					kvStr("event.name", "claude_code.user_prompt"),
					kvStr("session.id", "s1"),
					kvStr("prompt.id", "p1"),
				}},
				{TimeUnixNano: 2, Attributes: []*commonpb.KeyValue{
					kvStr("event.name", "claude_code.api_request"),
					kvStr("session.id", "s1"),
					kvStr("prompt.id", "p1"),
				}},
			}}},
		}},
	}
	if err := svc.IngestLogs(context.Background(), req); err != nil {
		t.Fatalf("IngestLogs: %v", err)
	}

	var n int
	if err := repo.DB().QueryRow("SELECT COUNT(*) FROM events").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("rows = %d, want 2", n)
	}
}

func TestService_IngestMetrics_StoresAllDatapoints(t *testing.T) {
	repo := openTempRepo(t)
	svc := New(repo, nil)

	req := &colmetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{{
			ScopeMetrics: []*metricspb.ScopeMetrics{{
				Metrics: []*metricspb.Metric{{
					Name: "claude_code.cost.usage",
					Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{
						DataPoints: []*metricspb.NumberDataPoint{
							{TimeUnixNano: 1, Attributes: []*commonpb.KeyValue{kvStr("session.id", "s1")}, Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: 0.05}},
						},
					}},
				}, {
					Name: "claude_code.token.usage",
					Data: &metricspb.Metric_Gauge{Gauge: &metricspb.Gauge{
						DataPoints: []*metricspb.NumberDataPoint{
							{TimeUnixNano: 2, Attributes: []*commonpb.KeyValue{kvStr("session.id", "s1")}, Value: &metricspb.NumberDataPoint_AsInt{AsInt: 1500}},
						},
					}},
				}},
			}},
		}},
	}
	if err := svc.IngestMetrics(context.Background(), req); err != nil {
		t.Fatalf("IngestMetrics: %v", err)
	}
	var n int
	_ = repo.DB().QueryRow("SELECT COUNT(*) FROM metric_snapshots").Scan(&n)
	if n != 2 {
		t.Fatalf("rows = %d, want 2", n)
	}
}

func TestService_IngestLogs_DropsRecordsMissingSessionID(t *testing.T) {
	repo := openTempRepo(t)
	svc := New(repo, nil)

	req := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{
			ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{
				{TimeUnixNano: 1, Attributes: []*commonpb.KeyValue{
					kvStr("event.name", "claude_code.user_prompt"),
					kvStr("session.id", "good"),
				}},
				{TimeUnixNano: 2, Attributes: []*commonpb.KeyValue{
					kvStr("event.name", "claude_code.user_prompt"),
					// no session.id → dropped
				}},
			}}},
		}},
	}
	if err := svc.IngestLogs(context.Background(), req); err != nil {
		t.Fatalf("IngestLogs: %v", err)
	}
	var n int
	_ = repo.DB().QueryRow("SELECT COUNT(*) FROM events").Scan(&n)
	if n != 1 {
		t.Fatalf("rows = %d, want 1 (one dropped)", n)
	}
}
