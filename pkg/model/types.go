package model

// NodeType classifies an identity graph node.
type NodeType string

const (
	NodeUser           NodeType = "user"
	NodeGroup          NodeType = "group"
	NodeComputer       NodeType = "computer"
	NodeServiceAccount NodeType = "service_account"
	NodeOU             NodeType = "ou"
	NodeGPO            NodeType = "gpo"
	NodeCA             NodeType = "certificate_authority"
	NodeCertTemplate   NodeType = "cert_template"
	NodeTrust          NodeType = "trust"
	NodeApplication    NodeType = "application"
	NodeSession        NodeType = "session"
	NodeRole           NodeType = "role"
	NodeTenant         NodeType = "tenant"
	NodeControl        NodeType = "control"
	NodeSecretStore    NodeType = "secret_store"
)

// EdgeType classifies the relationship between two nodes.
type EdgeType string

const (
	EdgeMemberOf           EdgeType = "member_of"
	EdgeAdminTo            EdgeType = "admin_to"
	EdgeLocalAdminTo       EdgeType = "local_admin_to"
	EdgeHasSessionOn       EdgeType = "has_session_on"
	EdgeCanDelegateTo      EdgeType = "can_delegate_to"
	EdgeCanSyncTo          EdgeType = "can_sync_to"
	EdgeCanEnrollIn        EdgeType = "can_enroll_in"
	EdgeTrustedBy          EdgeType = "trusted_by"
	EdgeInheritsPolicyFrom EdgeType = "inherits_policy_from"
	EdgeControlsGPO        EdgeType = "controls_gpo"
	EdgeCanResetPasswordOf EdgeType = "can_reset_password_of"
	EdgeCanWriteACLOf      EdgeType = "can_write_acl_of"
	EdgeAuthenticatesTo    EdgeType = "authenticates_to"
	EdgeSyncedTo           EdgeType = "synced_to"
	EdgeMonitoredBy        EdgeType = "monitored_by"
	EdgeProtectedBy        EdgeType = "protected_by"
	EdgePrivilegedOver     EdgeType = "privileged_over"
)

// Privilege tier tags used to mark sensitive targets.
const (
	TagTier0 = "tier0"
	TagTier1 = "tier1"
	TagTier2 = "tier2"
)
