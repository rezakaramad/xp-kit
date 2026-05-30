package nextinsight

// TenantMetadata holds the team and ART ownership data for a Next-Insight application.
// It is sourced from the /groups endpoint and describes who owns the tenant boundary.
type TenantMetadata struct {
	// AgileReleaseTrain is the first Agile Release Train group associated with the application.
	AgileReleaseTrain string

	// AgileTeam is the first Agile Team group associated with the application.
	AgileTeam string
}

// ApplicationMetadata holds the application-specific classification data for a Next-Insight application.
// It is sourced from the /applications/{id} endpoint and describes the workload itself.
type ApplicationMetadata struct {
	ApplicationID     string
	ApplicationName   string
	Lifecycle         string
	LifecycleDecision string
	Criticality       string
	Complexity        string
	Category          string
	DevelopmentType   string
	SourcingType      string
	FacingInternet    string
}

// TenantLabels returns Kubernetes-safe labels appropriate for Namespace objects
// and other resources that represent a tenant boundary.
// prefix is the label key prefix, e.g. "nextinsight.rezakara.demo/" — supplied by
// the caller so it can be configured per Composition without hardcoding it here.
//
// Returned keys (with given prefix):
//   - <prefix>agile-release-train
//   - <prefix>agile-team
func (m *TenantMetadata) TenantLabels(prefix string) map[string]string {
	labels := make(map[string]string)
	set(labels, prefix+"agile-release-train", normalize(m.AgileReleaseTrain))
	set(labels, prefix+"agile-team", normalize(m.AgileTeam))
	return labels
}

// WorkloadLabels returns Kubernetes-safe labels appropriate for Deployment, Service and similar workload resources inside a tenant namespace.
// prefix is the label key prefix like "nextinsight.rezakara.demo/" — supplied by the caller
func (m *ApplicationMetadata) WorkloadLabels(prefix string) map[string]string {
	labels := make(map[string]string)
	set(labels, prefix+"app-id", m.ApplicationID)
	set(labels, prefix+"app-name", normalize(m.ApplicationName))
	set(labels, prefix+"lifecycle", normalize(m.Lifecycle))
	set(labels, prefix+"lifecycle-decision", normalize(m.LifecycleDecision))
	set(labels, prefix+"criticality", normalize(m.Criticality))
	set(labels, prefix+"complexity", normalize(m.Complexity))
	set(labels, prefix+"category", normalize(m.Category))
	set(labels, prefix+"development-type", normalize(m.DevelopmentType))
	set(labels, prefix+"sourcing-type", normalize(m.SourcingType))
	set(labels, prefix+"facing-internet", m.FacingInternet)
	return labels
}
