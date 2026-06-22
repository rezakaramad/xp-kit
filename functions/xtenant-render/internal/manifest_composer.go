package render

import (
	"fmt"

	inputv1beta1 "github.com/rezakaramad/crosskit/functions/xtenant-render/input/v1beta1"
	"github.com/rezakaramad/crosskit/modules/nextinsight"
	runner "github.com/rezakaramad/crosskit/modules/runner"
	xtenant "github.com/rezakaramad/crosskit/types/xtenant"

	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
)

const (
	exportRepository   = "kubepave-tenants"
	exportRepoBranch   = "main"
	exportRepoBasePath = "tenants"

	baselineRepoURL      = "https://github.com/rezakaramad/kubepave"
	baselineRepoBranch   = "main"
	baselineRepoBasePath = "charts/baseline-tenant"

	gitopsRepoURL      = "https://github.com/rezakaramad/kubepave"
	gitopsRepoBranch   = "main"
	gitopsRepoBasePath = "charts/gitops-tenant"
)

// RepositoryFileComposer manages the single RepositoryFile child resource that
// bundles all rendered ArgoCD Applications into the GitOps export repository.
//
// It waits until every binding's Entra principal has an object ID in ctx.Observed
// before emitting the resource — returning nil from Compose signals "not ready yet".
type RepositoryFileComposer struct {
	nextInsight       nextinsight.Client
	lastRenderedCount int
}

func NewRepositoryFileComposer(ni nextinsight.Client) *RepositoryFileComposer {
	return &RepositoryFileComposer{nextInsight: ni}
}

// ConditionType reports the "Rendered" condition on the XR, mirroring the
// original function's behaviour.
func (c *RepositoryFileComposer) ConditionType() string { return "Rendered" }

func (c *RepositoryFileComposer) ResourceName(_ runner.Context[*xtenant.XTenant, *inputv1beta1.Input]) resource.Name {
	return "tenant-rendered-manifests"
}

// Compose resolves all principal object IDs, builds and bundles the ArgoCD
// Applications, enriches with Next-Insight labels, and returns the
// RepositoryFile resource. Returns nil when any principal is not yet available.
func (c *RepositoryFileComposer) Compose(ctx runner.Context[*xtenant.XTenant, *inputv1beta1.Input]) (*composed.Unstructured, error) {
	tenant := tenantSpecFromCtx(ctx)
	bindings := ctx.Input.Tenant.Bindings

	// Resolve all principal object IDs — return nil to wait if any are missing.
	resolvedBindings := make([]ResolvedBinding, 0, len(bindings))
	for _, binding := range bindings {
		objectID, ready := ResolveBindingPrincipalObjectID(ctx.Observed, ctx.Input.Azure, binding)
		if !ready {
			ctx.Log.Info("Waiting for principal object ID", "binding", binding.Name)
			return nil, runner.NewConditionError("WaitingForPrincipalObjectID", "waiting for principal object ID for binding "+binding.Name)
		}
		resolvedBindings = append(resolvedBindings, ResolvedBinding{
			Role:              binding.Name,
			Cluster:           binding.Cluster,
			EnvironmentPrefix: binding.EnvironmentPrefix,
			PrincipalObjectID: objectID,
		})
	}

	clusters := UniqueClustersFromBindings(bindings)

	// Build ArgoCD Applications.
	baselineApps, err := BuildBaselineApplications(
		tenant, clusters,
		baselineRepoURL, baselineRepoBranch, baselineRepoBasePath,
	)
	if err != nil {
		return nil, fmt.Errorf("building baseline applications: %w", err)
	}

	gitopsApp, err := BuildGitopsApplication(
		tenant, resolvedBindings, ctx.Input.Azure,
		gitopsRepoURL, gitopsRepoBranch, gitopsRepoBasePath,
	)
	if err != nil {
		return nil, fmt.Errorf("building gitops application: %w", err)
	}

	// Enrich with Next-Insight labels (non-fatal if unavailable).
	nextInsightLabels, err := FetchTenantLabels(ctx.Ctx, c.nextInsight, tenant.Spec.TeamID, ctx.Input.NextInsight.LabelPrefix)
	if err != nil {
		ctx.Log.Info("Skipping Next-Insight label enrichment", "error", err)
		nextInsightLabels = map[string]string{}
	}
	ApplyNextInsightLabels(nextInsightLabels, append(baselineApps, gitopsApp)...)

	// Bundle all apps into a single YAML and wrap in a RepositoryFile.
	resources := make([]*composed.Unstructured, 0, 1+len(baselineApps))
	resources = append(resources, gitopsApp)
	resources = append(resources, baselineApps...)
	c.lastRenderedCount = len(resources)

	content, err := BundleYAML(resources...)
	if err != nil {
		return nil, fmt.Errorf("bundling resources: %w", err)
	}

	github := ctx.Input.Github.WithDefaults()
	return BuildRepositoryFile(tenant, content, RepositoryFileConfig{
		ProviderConfigName: github.ProviderConfigName,
		Repository:         exportRepository,
		Branch:             exportRepoBranch,
		BasePath:           exportRepoBasePath,
		CommitAuthor:       github.CommitAuthor,
		CommitEmail:        github.CommitEmail,
	}), nil
}

// IsReady returns true as soon as the RepositoryFile resource is emitted.
// We don't wait for observed confirmation — the resource being pushed is the desired outcome.
func (c *RepositoryFileComposer) IsReady(_ runner.Context[*xtenant.XTenant, *inputv1beta1.Input], _ *composed.Unstructured) bool {
	return true
}

func (c *RepositoryFileComposer) ConnectionDetails(_ runner.Context[*xtenant.XTenant, *inputv1beta1.Input], _ *composed.Unstructured) map[string]string {
	return nil
}

// ReadyMessage implements the optional runner.ResultMessager interface.
// It is called by the runner when the RepositoryFile resource is ready, and its
// return value is attached to the Rendered=True condition on the XR.
func (c *RepositoryFileComposer) ReadyMessage(ctx runner.Context[*xtenant.XTenant, *inputv1beta1.Input]) string {
	return fmt.Sprintf("Rendered %d resources for tenant %q", c.lastRenderedCount, ctx.XR.GetName())
}
