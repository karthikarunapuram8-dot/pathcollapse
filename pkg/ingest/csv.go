package ingest

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/karunapuram/pathcollapse/pkg/model"
)

// CSVUserAdapter parses a CSV with columns: id,name,type,tags
type CSVUserAdapter struct{}

func (a *CSVUserAdapter) Name() string { return "csv-users" }

func (a *CSVUserAdapter) Ingest(r io.Reader) (*Result, error) {
	records, err := readCSV(r)
	if err != nil {
		return nil, fmt.Errorf("csv_users: %w", err)
	}
	res := &Result{}
	now := time.Now().UTC()
	for i, row := range records {
		if i == 0 {
			continue // skip header
		}
		if len(row) < 2 {
			res.Warns = append(res.Warns, fmt.Sprintf("row %d: too few columns", i+1))
			continue
		}
		id := strings.TrimSpace(row[0])
		name := strings.TrimSpace(row[1])
		nodeType := model.NodeUser
		if len(row) >= 3 && strings.TrimSpace(row[2]) != "" {
			nodeType = model.NodeType(strings.TrimSpace(row[2]))
		}
		n := model.NewNode(id, nodeType, name)
		n.FirstSeen = now
		n.LastSeen = now
		if len(row) >= 4 {
			for _, tag := range strings.Split(row[3], ";") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					n.Tags = append(n.Tags, tag)
				}
			}
		}
		res.Nodes = append(res.Nodes, n)
	}
	return res, nil
}

// CSVGroupAdapter parses group membership: member_id,group_id
type CSVGroupAdapter struct{}

func (a *CSVGroupAdapter) Name() string { return "csv-groups" }

func (a *CSVGroupAdapter) Ingest(r io.Reader) (*Result, error) {
	records, err := readCSV(r)
	if err != nil {
		return nil, fmt.Errorf("csv_groups: %w", err)
	}
	res := &Result{}
	seq := 0
	for i, row := range records {
		if i == 0 {
			continue
		}
		if len(row) < 2 {
			continue
		}
		member := strings.TrimSpace(row[0])
		group := strings.TrimSpace(row[1])
		if member == "" || group == "" {
			continue
		}
		e := model.NewEdge(fmt.Sprintf("csv-grp-%d", seq), model.EdgeMemberOf, member, group)
		seq++
		res.Edges = append(res.Edges, e)
	}
	return res, nil
}

// CSVLocalAdminAdapter parses local admin relationships: user_id,computer_id,confidence
type CSVLocalAdminAdapter struct{}

func (a *CSVLocalAdminAdapter) Name() string { return "csv-local-admin" }

func (a *CSVLocalAdminAdapter) Ingest(r io.Reader) (*Result, error) {
	records, err := readCSV(r)
	if err != nil {
		return nil, fmt.Errorf("csv_local_admin: %w", err)
	}
	res := &Result{}
	seq := 0
	for i, row := range records {
		if i == 0 {
			continue
		}
		if len(row) < 2 {
			continue
		}
		user := strings.TrimSpace(row[0])
		computer := strings.TrimSpace(row[1])
		conf := 1.0
		if len(row) >= 3 {
			if v, err := strconv.ParseFloat(strings.TrimSpace(row[2]), 64); err == nil {
				conf = v
			}
		}
		e := model.NewEdge(fmt.Sprintf("csv-la-%d", seq), model.EdgeLocalAdminTo, user, computer)
		e.Confidence = conf
		seq++
		res.Edges = append(res.Edges, e)
	}
	return res, nil
}

// CSVGPOAdapter parses GPO link relationships: gpo_id,ou_id,gpo_name
type CSVGPOAdapter struct{}

func (a *CSVGPOAdapter) Name() string { return "csv-gpo" }

func (a *CSVGPOAdapter) Ingest(r io.Reader) (*Result, error) {
	records, err := readCSV(r)
	if err != nil {
		return nil, fmt.Errorf("csv_gpo: %w", err)
	}
	res := &Result{}
	now := time.Now().UTC()
	seq := 0
	for i, row := range records {
		if i == 0 {
			continue
		}
		if len(row) < 2 {
			continue
		}
		gpoID := strings.TrimSpace(row[0])
		ouID := strings.TrimSpace(row[1])
		gpoName := gpoID
		if len(row) >= 3 {
			gpoName = strings.TrimSpace(row[2])
		}

		gpoNode := model.NewNode(gpoID, model.NodeGPO, gpoName)
		gpoNode.FirstSeen = now
		gpoNode.LastSeen = now
		res.Nodes = append(res.Nodes, gpoNode)

		e := model.NewEdge(fmt.Sprintf("csv-gpo-%d", seq), model.EdgeControlsGPO, gpoID, ouID)
		seq++
		res.Edges = append(res.Edges, e)
	}
	return res, nil
}

func readCSV(r io.Reader) ([][]string, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1
	return cr.ReadAll()
}
