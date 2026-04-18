package ingest

import (
	"fmt"
	"io"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/karunapuram/pathcollapse/pkg/model"
)

// yamlFact is the wire format for analyst-provided YAML facts.
type yamlFact struct {
	Nodes []yamlNode `yaml:"nodes"`
	Edges []yamlEdge `yaml:"edges"`
}

type yamlNode struct {
	ID         string         `yaml:"id"`
	Type       string         `yaml:"type"`
	Name       string         `yaml:"name"`
	Tags       []string       `yaml:"tags"`
	Attributes map[string]any `yaml:"attributes"`
}

type yamlEdge struct {
	ID             string   `yaml:"id"`
	Type           string   `yaml:"type"`
	Source         string   `yaml:"source"`
	Target         string   `yaml:"target"`
	Confidence     *float64 `yaml:"confidence"`
	Exploitability *float64 `yaml:"exploitability"`
	Detectability  *float64 `yaml:"detectability"`
	BlastRadius    *float64 `yaml:"blast_radius"`
}

// YAMLAdapter parses analyst-provided YAML fact files.
type YAMLAdapter struct{}

func (a *YAMLAdapter) Name() string { return "yaml-facts" }

func (a *YAMLAdapter) Ingest(r io.Reader) (*Result, error) {
	var fact yamlFact
	dec := yaml.NewDecoder(r)
	if err := dec.Decode(&fact); err != nil {
		return nil, fmt.Errorf("yaml adapter: decode: %w", err)
	}

	res := &Result{}
	now := time.Now().UTC()

	for _, yn := range fact.Nodes {
		if yn.ID == "" {
			res.Warns = append(res.Warns, "skipping yaml node with empty ID")
			continue
		}
		n := model.NewNode(yn.ID, model.NodeType(yn.Type), yn.Name)
		n.Tags = yn.Tags
		if yn.Attributes != nil {
			n.Attributes = yn.Attributes
		}
		n.FirstSeen = now
		n.LastSeen = now
		res.Nodes = append(res.Nodes, n)
	}

	for _, ye := range fact.Edges {
		if ye.Source == "" || ye.Target == "" {
			res.Warns = append(res.Warns, fmt.Sprintf("skipping yaml edge with missing endpoints: %v->%v", ye.Source, ye.Target))
			continue
		}
		id := ye.ID
		if id == "" {
			id = fmt.Sprintf("yaml-%s-%s", ye.Source, ye.Target)
		}
		e := model.NewEdge(id, model.EdgeType(ye.Type), ye.Source, ye.Target)
		if ye.Confidence != nil {
			e.Confidence = *ye.Confidence
		}
		if ye.Exploitability != nil {
			e.Exploitability = *ye.Exploitability
		}
		if ye.Detectability != nil {
			e.Detectability = *ye.Detectability
		}
		if ye.BlastRadius != nil {
			e.BlastRadius = *ye.BlastRadius
		}
		res.Edges = append(res.Edges, e)
	}

	return res, nil
}
