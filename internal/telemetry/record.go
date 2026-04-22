// Package telemetry emits structured events to a local log file in
// OpenTelemetry log-record shape. Records are JSON-lines, rotated by size
// to bound disk usage, and safe to ship later via Filebeat / an OTLP HTTP
// sink without changing the call sites.
//
// No credentials or vault data are ever passed through this package; callers
// must only pass coarse attributes (counts, category IDs, booleans).
package telemetry

import "time"

// Severity mirrors the subset of OTel severity levels we emit.
// Numbers match the OTel spec so downstream tools index correctly.
type Severity int

const (
	SeverityDebug Severity = 5
	SeverityInfo  Severity = 9
	SeverityWarn  Severity = 13
	SeverityError Severity = 17
)

func (s Severity) Text() string {
	switch s {
	case SeverityDebug:
		return "DEBUG"
	case SeverityWarn:
		return "WARN"
	case SeverityError:
		return "ERROR"
	default:
		return "INFO"
	}
}

// Record is one emitted event. Field names match the OTel log record schema
// so events drop straight into Elasticsearch / OpenSearch / Loki without
// transformation. Attributes hold event-specific fields; Resource holds
// session-wide fields (set once at Init).
type Record struct {
	Timestamp      time.Time              `json:"timestamp"`
	SeverityText   string                 `json:"severityText"`
	SeverityNumber int                    `json:"severityNumber"`
	Body           string                 `json:"body"` // event name, e.g. "app.start"
	Attributes     map[string]interface{} `json:"attributes,omitempty"`
	Resource       map[string]interface{} `json:"resource"`
}
