// Package testdata provides a realistic enterprise Active Directory fixture:
// 50 users, 20 groups, 10 computers, 5 service accounts, GPOs, trusts,
// and cert templates — with realistic privilege paths.
package testdata

import (
	"fmt"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
)

// EnterpriseAD builds and returns the fixture graph.
func EnterpriseAD() *graph.Graph {
	g := graph.New()

	addNodes(g)
	addEdges(g)

	return g
}

func addNodes(g *graph.Graph) {
	// 20 groups
	groups := []struct {
		id   string
		name string
		tags []string
	}{
		{"grp-domain-admins", "Domain Admins", []string{model.TagTier0}},
		{"grp-enterprise-admins", "Enterprise Admins", []string{model.TagTier0}},
		{"grp-schema-admins", "Schema Admins", []string{model.TagTier0}},
		{"grp-backup-operators", "Backup Operators", []string{model.TagTier0}},
		{"grp-account-operators", "Account Operators", []string{model.TagTier1}},
		{"grp-server-admins", "Server Admins", []string{model.TagTier1}},
		{"grp-helpdesk", "Help Desk", []string{model.TagTier2}},
		{"grp-developers", "Developers", []string{model.TagTier2}},
		{"grp-finance", "Finance", nil},
		{"grp-hr", "HR", nil},
		{"grp-marketing", "Marketing", nil},
		{"grp-sales", "Sales", nil},
		{"grp-it-ops", "IT Ops", []string{model.TagTier1}},
		{"grp-security", "Security Team", []string{model.TagTier1}},
		{"grp-read-only-dcs", "Read-Only DCs", []string{model.TagTier0}},
		{"grp-cert-publishers", "Cert Publishers", []string{model.TagTier1}},
		{"grp-dns-admins", "DNS Admins", []string{model.TagTier0}},
		{"grp-remote-desktop", "Remote Desktop Users", nil},
		{"grp-guests", "Guests", nil},
		{"grp-contractors", "Contractors", nil},
	}
	for _, grp := range groups {
		n := model.NewNode(grp.id, model.NodeGroup, grp.name)
		n.Tags = grp.tags
		g.AddNode(n)
	}

	// 10 computers
	computers := []struct {
		id   string
		name string
		tags []string
	}{
		{"cmp-dc01", "DC01", []string{model.TagTier0}},
		{"cmp-dc02", "DC02", []string{model.TagTier0}},
		{"cmp-adcs01", "ADCS01", []string{model.TagTier0}}, // CA server
		{"cmp-fileserver01", "FILESERVER01", []string{model.TagTier1}},
		{"cmp-sqlserver01", "SQLSERVER01", []string{model.TagTier1}},
		{"cmp-webserver01", "WEBSERVER01", []string{model.TagTier2}},
		{"cmp-ws-alice", "WS-ALICE", nil},
		{"cmp-ws-bob", "WS-BOB", nil},
		{"cmp-ws-charlie", "WS-CHARLIE", nil},
		{"cmp-jumpbox01", "JUMPBOX01", []string{model.TagTier1}},
	}
	for _, cmp := range computers {
		n := model.NewNode(cmp.id, model.NodeComputer, cmp.name)
		n.Tags = cmp.tags
		g.AddNode(n)
	}

	// 5 service accounts
	svcAccounts := []struct {
		id   string
		name string
	}{
		{"svc-sql", "svc-sql"},
		{"svc-web", "svc-web"},
		{"svc-backup", "svc-backup"},
		{"svc-monitor", "svc-monitor"},
		{"svc-deploy", "svc-deploy"},
	}
	for _, svc := range svcAccounts {
		n := model.NewNode(svc.id, model.NodeServiceAccount, svc.name)
		g.AddNode(n)
	}

	// GPO nodes
	gpos := []string{"gpo-domain-policy", "gpo-workstation-policy", "gpo-dc-policy"}
	for _, gid := range gpos {
		g.AddNode(model.NewNode(gid, model.NodeGPO, gid))
	}

	// OUs
	ous := []string{"ou-users", "ou-computers", "ou-servers"}
	for _, ouid := range ous {
		g.AddNode(model.NewNode(ouid, model.NodeOU, ouid))
	}

	// CA and cert template
	g.AddNode(model.NewNode("ca-corp", model.NodeCA, "CORP-CA"))
	certTemplate := model.NewNode("cert-tmpl-user", model.NodeCertTemplate, "User Cert Template")
	certTemplate.Attributes["enrollee_supplies_subject"] = true
	certTemplate.Attributes["client_authentication"] = true
	g.AddNode(certTemplate)

	// Trust
	g.AddNode(model.NewNode("trust-partner", model.NodeTrust, "Partner Domain Trust"))

	// 50 users: 5 privileged + 45 regular
	privilegedUsers := []struct {
		id   string
		name string
	}{
		{"usr-alice", "alice"},
		{"usr-bob", "bob"},
		{"usr-charlie", "charlie"},
		{"usr-helpdesk1", "helpdesk1"},
		{"usr-helpdesk2", "helpdesk2"},
	}
	for _, u := range privilegedUsers {
		n := model.NewNode(u.id, model.NodeUser, u.name)
		g.AddNode(n)
	}
	for i := 1; i <= 45; i++ {
		uid := fmt.Sprintf("usr-regular%02d", i)
		n := model.NewNode(uid, model.NodeUser, fmt.Sprintf("user%02d", i))
		g.AddNode(n)
	}
}

func addEdges(g *graph.Graph) {
	seq := 0
	addEdge := func(typ model.EdgeType, src, tgt string, conf, exploit, detect, blast float64) {
		e := model.NewEdge(fmt.Sprintf("fix-%d", seq), typ, src, tgt)
		e.Confidence = conf
		e.Exploitability = exploit
		e.Detectability = detect
		e.BlastRadius = blast
		seq++
		g.AddEdge(e)
	}

	// Group memberships — privileged paths
	addEdge(model.EdgeMemberOf, "usr-alice", "grp-domain-admins", 1.0, 0.9, 0.3, 0.9)
	addEdge(model.EdgeMemberOf, "usr-bob", "grp-server-admins", 1.0, 0.7, 0.4, 0.7)
	addEdge(model.EdgeMemberOf, "usr-charlie", "grp-account-operators", 1.0, 0.6, 0.5, 0.6)
	addEdge(model.EdgeMemberOf, "usr-helpdesk1", "grp-helpdesk", 1.0, 0.4, 0.6, 0.4)
	addEdge(model.EdgeMemberOf, "usr-helpdesk2", "grp-helpdesk", 1.0, 0.4, 0.6, 0.4)

	// Dangerous: helpdesk can reset passwords of account operators
	addEdge(model.EdgeCanResetPasswordOf, "grp-helpdesk", "grp-account-operators", 0.9, 0.7, 0.3, 0.7)
	// Account operators can write ACL of domain admins — dangerous path
	addEdge(model.EdgeCanWriteACLOf, "grp-account-operators", "grp-domain-admins", 0.8, 0.8, 0.2, 0.9)

	// Domain admins admin to DCs
	addEdge(model.EdgeAdminTo, "grp-domain-admins", "cmp-dc01", 1.0, 1.0, 0.2, 1.0)
	addEdge(model.EdgeAdminTo, "grp-domain-admins", "cmp-dc02", 1.0, 1.0, 0.2, 1.0)
	addEdge(model.EdgeAdminTo, "grp-domain-admins", "cmp-adcs01", 1.0, 1.0, 0.2, 1.0)

	// Server admins to servers
	addEdge(model.EdgeAdminTo, "grp-server-admins", "cmp-fileserver01", 1.0, 0.8, 0.4, 0.7)
	addEdge(model.EdgeAdminTo, "grp-server-admins", "cmp-sqlserver01", 1.0, 0.8, 0.4, 0.7)

	// Service account delegation (risky)
	addEdge(model.EdgeCanDelegateTo, "svc-sql", "cmp-dc01", 0.9, 0.8, 0.2, 0.9) // Unconstrained!
	addEdge(model.EdgeAdminTo, "svc-sql", "cmp-sqlserver01", 1.0, 0.7, 0.4, 0.6)

	// svc-backup can sync (DCSync risk)
	addEdge(model.EdgeCanSyncTo, "svc-backup", "cmp-dc01", 0.7, 0.9, 0.2, 1.0)

	// svc-deploy has local admin to workstations
	addEdge(model.EdgeLocalAdminTo, "svc-deploy", "cmp-ws-alice", 1.0, 0.6, 0.5, 0.4)
	addEdge(model.EdgeLocalAdminTo, "svc-deploy", "cmp-ws-bob", 1.0, 0.6, 0.5, 0.4)

	// Sessions (lateral movement paths)
	addEdge(model.EdgeHasSessionOn, "usr-alice", "cmp-ws-alice", 0.8, 0.7, 0.5, 0.5)
	addEdge(model.EdgeHasSessionOn, "usr-bob", "cmp-ws-bob", 0.8, 0.7, 0.5, 0.5)

	// Certificate template enrollment
	addEdge(model.EdgeCanEnrollIn, "grp-domain-admins", "cert-tmpl-user", 0.9, 0.8, 0.3, 0.8)
	addEdge(model.EdgeCanEnrollIn, "grp-developers", "cert-tmpl-user", 0.9, 0.7, 0.4, 0.7)

	// CA controls
	addEdge(model.EdgeAdminTo, "grp-cert-publishers", "ca-corp", 0.9, 0.8, 0.3, 0.8)

	// GPO links
	addEdge(model.EdgeControlsGPO, "gpo-dc-policy", "ou-computers", 1.0, 0.5, 0.6, 0.5)

	// Trust
	addEdge(model.EdgeTrustedBy, "trust-partner", "grp-domain-admins", 0.6, 0.5, 0.3, 0.6)

	// Regular users in benign groups
	for i := 1; i <= 15; i++ {
		uid := fmt.Sprintf("usr-regular%02d", i)
		addEdge(model.EdgeMemberOf, uid, "grp-finance", 1.0, 0.2, 0.7, 0.2)
	}
	for i := 16; i <= 30; i++ {
		uid := fmt.Sprintf("usr-regular%02d", i)
		addEdge(model.EdgeMemberOf, uid, "grp-hr", 1.0, 0.2, 0.7, 0.2)
	}
	for i := 31; i <= 45; i++ {
		uid := fmt.Sprintf("usr-regular%02d", i)
		addEdge(model.EdgeMemberOf, uid, "grp-marketing", 1.0, 0.2, 0.7, 0.2)
	}
	// Developers group
	addEdge(model.EdgeMemberOf, "usr-regular01", "grp-developers", 1.0, 0.3, 0.6, 0.3)
	addEdge(model.EdgeMemberOf, "usr-regular02", "grp-developers", 1.0, 0.3, 0.6, 0.3)

	// Remote desktop access for some users
	addEdge(model.EdgeMemberOf, "usr-regular01", "grp-remote-desktop", 1.0, 0.3, 0.6, 0.2)
}
