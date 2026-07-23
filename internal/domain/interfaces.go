package domain

import "context"

// Collector is the interface all system-data collectors implement.
// Each module (CPU, Memory, Disk, …) provides its own implementation.
type Collector interface {
	// Name returns a short human-readable name of what this collects.
	Name() string

	// Collect gathers data and returns it. Implementations MUST be
	// safe to call from multiple goroutines (or the caller serialises).
	Collect(ctx context.Context) (any, error)
}

// Exporter serialises a Report to a specific output format.
type Exporter interface {
	// Extension returns the file extension this exporter produces (e.g. "json").
	Extension() string

	// Export writes the report to a byte slice.
	Export(report *Report) ([]byte, error)
}

// Scorer computes scores for the report.
type Scorer interface {
	ComputeHealth(report *Report) Score
	ComputeSecurity(report *Report) Score
}

// Comparer produces a diff between two reports.
type Comparer interface {
	Compare(a, b *Report) []CompareResult
}

// CollectorRegistry allows dynamic registration of collectors.
type CollectorRegistry interface {
	Register(c Collector)
	Collectors() []Collector
}
