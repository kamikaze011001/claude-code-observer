package projectinit

const (
	KeyEnableTelemetry      = "CLAUDE_CODE_ENABLE_TELEMETRY"
	KeyMetricsExporter      = "OTEL_METRICS_EXPORTER"
	KeyLogsExporter         = "OTEL_LOGS_EXPORTER"
	KeyOTLPProtocol         = "OTEL_EXPORTER_OTLP_PROTOCOL"
	KeyOTLPEndpoint         = "OTEL_EXPORTER_OTLP_ENDPOINT"
	KeyResourceAttrs        = "OTEL_RESOURCE_ATTRIBUTES"
	KeyMetricExportInterval = "OTEL_METRIC_EXPORT_INTERVAL"

	SubKeyProjectName = "project.name"
)

// CanonicalValues returns the value cco init writes for each owned key
// EXCEPT KeyResourceAttrs, which is merged sub-key-wise (see resource_attrs.go).
func CanonicalValues() map[string]string {
	return map[string]string{
		KeyEnableTelemetry:      "1",
		KeyMetricsExporter:      "otlp",
		KeyLogsExporter:         "otlp",
		KeyOTLPProtocol:         "grpc",
		KeyOTLPEndpoint:         "http://localhost:4317",
		KeyMetricExportInterval: "20000",
	}
}

// OwnedKeys returns the full owned set in stable insertion order. Useful for
// emitting a fresh env block.
func OwnedKeys() []string {
	return []string{
		KeyEnableTelemetry,
		KeyMetricsExporter,
		KeyLogsExporter,
		KeyOTLPProtocol,
		KeyOTLPEndpoint,
		KeyResourceAttrs,
		KeyMetricExportInterval,
	}
}
