// Package policy evaluates policy rules against graph state.
package policy

import (
	"fmt"

	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/model"
)

// RuleType classifies a policy rule.
type RuleType string

const (
	RuleNoPrivilegedMembership RuleType = "no_privileged_membership"
	RuleNoUnconstrained        RuleType = "no_unconstrained_delegation"
	RuleServiceAccountIsolation RuleType = "service_account_isolation"
)

// Rule is a single policy constraint on graph state.
type Rule struct {
	ID          string   `json:"id"`
	Type        RuleType `json:"type"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
}

// Violation is a rule breach found in the graph.
type Violation struct {
	Rule    Rule   `json:"rule"`
	NodeID  string `json:"node_id,omitempty"`
	EdgeID  string `json:"edge_id,omitempty"`
	Detail  string `json:"detail"`
}

// EvaluationResult holds all violations found.
type EvaluationResult struct {
	Violations []Violation `json:"violations"`
	Passed     int         `json:"passed"`
}

// Evaluator checks graph state against a set of rules.
type Evaluator struct {
	rules []Rule
}

// NewEvaluator returns an Evaluator with the given rules.
func NewEvaluator(rules []Rule) *Evaluator {
	return &Evaluator{rules: rules}
}

// DefaultRules returns a baseline policy rule set.
func DefaultRules() []Rule {
	return []Rule{
		{ID: "P001", Type: RuleNoUnconstrained, Description: "No unconstrained Kerberos delegation", Severity: "critical"},
		{ID: "P002", Type: RuleServiceAccountIsolation, Description: "Service accounts must not have interactive logon rights", Severity: "high"},
	}
}

// Evaluate checks the graph against all rules and returns violations.
func (ev *Evaluator) Evaluate(g *graph.Graph) *EvaluationResult {
	res := &EvaluationResult{}
	for _, rule := range ev.rules {
		violations := ev.evalRule(rule, g)
		res.Violations = append(res.Violations, violations...)
		if len(violations) == 0 {
			res.Passed++
		}
	}
	return res
}

func (ev *Evaluator) evalRule(rule Rule, g *graph.Graph) []Violation {
	switch rule.Type {
	case RuleNoUnconstrained:
		return ev.checkUnconstrained(rule, g)
	case RuleServiceAccountIsolation:
		return ev.checkServiceAccountIsolation(rule, g)
	default:
		return nil
	}
}

func (ev *Evaluator) checkUnconstrained(rule Rule, g *graph.Graph) []Violation {
	var violations []Violation
	for _, e := range g.Edges() {
		if e.Type == model.EdgeCanDelegateTo {
			src := g.GetNode(e.Source)
			name := e.Source
			if src != nil {
				name = src.Name
			}
			violations = append(violations, Violation{
				Rule:   rule,
				EdgeID: e.ID,
				Detail: fmt.Sprintf("%s has unconstrained delegation to %s", name, e.Target),
			})
		}
	}
	return violations
}

func (ev *Evaluator) checkServiceAccountIsolation(rule Rule, g *graph.Graph) []Violation {
	var violations []Violation
	for _, e := range g.Edges() {
		if e.Type != model.EdgeHasSessionOn {
			continue
		}
		src := g.GetNode(e.Source)
		if src == nil || src.Type != model.NodeServiceAccount {
			continue
		}
		violations = append(violations, Violation{
			Rule:   rule,
			EdgeID: e.ID,
			Detail: fmt.Sprintf("Service account %s has interactive session on %s", src.Name, e.Target),
		})
	}
	return violations
}
