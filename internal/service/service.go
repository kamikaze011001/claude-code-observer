// Package service is the orchestration layer between the OTLP receiver and
// the SQLite repository. It owns the parse-and-write transaction boundary.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
	"github.com/kamikaze011001/claude-code-observer/internal/eventparser"
	"github.com/kamikaze011001/claude-code-observer/internal/repository"
)

// Service satisfies receiver.LogIngester and receiver.MetricIngester.
type Service struct {
	repo *repository.Repository
	log  *slog.Logger
}

// New constructs a Service. A nil logger means slog.Default().
func New(repo *repository.Repository, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, log: log}
}

// IngestLogs implements receiver.LogIngester.
func (s *Service) IngestLogs(ctx context.Context, req *collogspb.ExportLogsServiceRequest) error {
	events := make([]domain.Event, 0, 16)
	for _, rl := range req.GetResourceLogs() {
		res := rl.GetResource()
		for _, sl := range rl.GetScopeLogs() {
			for _, rec := range sl.GetLogRecords() {
				ev, err := eventparser.Parse(rec, res)
				if err != nil {
					if errors.Is(err, eventparser.ErrDrop) {
						s.log.Warn("dropping log record", "err", err)
						continue
					}
					return fmt.Errorf("parse log record: %w", err)
				}
				events = append(events, ev)
			}
		}
	}
	if err := s.repo.InsertEvents(ctx, events); err != nil {
		return fmt.Errorf("insert events: %w", err)
	}
	return nil
}

// IngestMetrics implements receiver.MetricIngester. It walks every metric
// datapoint and persists each one verbatim.
func (s *Service) IngestMetrics(ctx context.Context, req *colmetricspb.ExportMetricsServiceRequest) error {
	snaps := make([]domain.MetricSnapshot, 0, 16)
	for _, rm := range req.GetResourceMetrics() {
		res := rm.GetResource()
		for _, sm := range rm.GetScopeMetrics() {
			for _, m := range sm.GetMetrics() {
				snaps = append(snaps, datapointsToSnapshots(m, res)...)
			}
		}
	}
	if err := s.repo.InsertMetricSnapshots(ctx, snaps); err != nil {
		return fmt.Errorf("insert metric snapshots: %w", err)
	}
	return nil
}

// datapointsToSnapshots flattens every datapoint of every metric type into
// MetricSnapshot rows. Histograms are stored as their .Sum value; rollups in
// Phase 2 only consume Sum/Gauge series anyway.
func datapointsToSnapshots(m *metricspb.Metric, res *resourcepb.Resource) []domain.MetricSnapshot {
	resAttrs := flattenResourceAttrs(res)
	out := []domain.MetricSnapshot{}
	switch d := m.GetData().(type) {
	case *metricspb.Metric_Sum:
		for _, dp := range d.Sum.GetDataPoints() {
			out = append(out, snapFromNumber(m.GetName(), dp, resAttrs))
		}
	case *metricspb.Metric_Gauge:
		for _, dp := range d.Gauge.GetDataPoints() {
			out = append(out, snapFromNumber(m.GetName(), dp, resAttrs))
		}
	case *metricspb.Metric_Histogram:
		for _, dp := range d.Histogram.GetDataPoints() {
			out = append(out, domain.MetricSnapshot{
				TS:         int64(dp.GetTimeUnixNano()),
				SessionID:  sessionFromKVs(dp.GetAttributes()),
				MetricName: m.GetName(),
				Value:      dp.GetSum(),
				Attrs:      mergeAttrs(eventparser.FlattenKVs(dp.GetAttributes()), resAttrs),
			})
		}
	}
	return out
}

func snapFromNumber(name string, dp *metricspb.NumberDataPoint, resAttrs map[string]any) domain.MetricSnapshot {
	var v float64
	switch x := dp.GetValue().(type) {
	case *metricspb.NumberDataPoint_AsDouble:
		v = x.AsDouble
	case *metricspb.NumberDataPoint_AsInt:
		v = float64(x.AsInt)
	}
	return domain.MetricSnapshot{
		TS:         int64(dp.GetTimeUnixNano()),
		SessionID:  sessionFromKVs(dp.GetAttributes()),
		MetricName: name,
		Value:      v,
		Attrs:      mergeAttrs(eventparser.FlattenKVs(dp.GetAttributes()), resAttrs),
	}
}

func sessionFromKVs(kvs []*commonpb.KeyValue) string {
	for _, kv := range kvs {
		if kv.GetKey() == "session.id" {
			if s, ok := kv.GetValue().GetValue().(*commonpb.AnyValue_StringValue); ok {
				return s.StringValue
			}
		}
	}
	return ""
}

func mergeAttrs(a, b map[string]any) map[string]any {
	if a == nil && b == nil {
		return nil
	}
	out := make(map[string]any, len(a)+len(b))
	for k, v := range b {
		out[k] = v
	}
	for k, v := range a {
		out[k] = v
	}
	return out
}

func flattenResourceAttrs(res *resourcepb.Resource) map[string]any {
	if res == nil {
		return nil
	}
	return eventparser.FlattenKVs(res.GetAttributes())
}
