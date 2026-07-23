package exporter

import (
	"encoding/json"
	"fmt"

	"sysscope/internal/domain"
)

// JSONExporter exports a report as pretty-printed JSON.
type JSONExporter struct{}

func NewJSON() *JSONExporter { return &JSONExporter{} }

func (e *JSONExporter) Extension() string { return "json" }

func (e *JSONExporter) Export(report *domain.Report) ([]byte, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("json export: %w", err)
	}
	return data, nil
}
