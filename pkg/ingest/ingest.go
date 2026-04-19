// Package ingest provides read-only adapters that parse identity data
// from various formats into model.Node and model.Edge collections.
package ingest

import (
	"fmt"
	"io"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
)

// Result holds everything produced by a successful ingest.
type Result struct {
	Nodes []*model.Node
	Edges []*model.Edge
	Warns []string
}

// Adapter is the common interface all ingestion adapters implement.
type Adapter interface {
	// Name returns a short human-readable label for the adapter.
	Name() string
	// Ingest reads r and returns the parsed result.
	Ingest(r io.Reader) (*Result, error)
}

// AdapterType selects a registered adapter.
type AdapterType string

const (
	AdapterGenericJSON   AdapterType = "json"
	AdapterCSVUsers      AdapterType = "csv_users"
	AdapterCSVGroups     AdapterType = "csv_groups"
	AdapterCSVLocalAdmin AdapterType = "csv_local_admin"
	AdapterCSVGPO        AdapterType = "csv_gpo"
	AdapterBloodHound    AdapterType = "bloodhound"
	AdapterYAMLFacts     AdapterType = "yaml"
)

// Get returns the adapter for the given type.
func Get(t AdapterType) (Adapter, error) {
	switch t {
	case AdapterGenericJSON:
		return &JSONAdapter{}, nil
	case AdapterCSVUsers:
		return &CSVUserAdapter{}, nil
	case AdapterCSVGroups:
		return &CSVGroupAdapter{}, nil
	case AdapterCSVLocalAdmin:
		return &CSVLocalAdminAdapter{}, nil
	case AdapterCSVGPO:
		return &CSVGPOAdapter{}, nil
	case AdapterBloodHound:
		return &BloodHoundAdapter{}, nil
	case AdapterYAMLFacts:
		return &YAMLAdapter{}, nil
	default:
		return nil, fmt.Errorf("ingest: unknown adapter type %q", t)
	}
}
