package main

import (
	"context"
	"fmt"
	"maps"

	inputv1beta1 "github.com/rezakaramad/crossplane-toolkit/functions/xtenant-render/input/v1beta1"
	render "github.com/rezakaramad/crossplane-toolkit/functions/xtenant-render/internal"
	"github.com/rezakaramad/crossplane-toolkit/modules/nextinsight"
	xtenant "github.com/rezakaramad/crossplane-toolkit/types/xtenant"
	"k8s.io/apimachinery/pkg/runtime"

	xperrors "github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/crossplane/function-sdk-go/response"
)

const conditionRendered = "Rendered"

const (
	exportRepository   = "https://github.com/rezakaramad/kubepave-tenants"
	exportRepoBranch   = "main"
	exportRepoBasePath = "tenants"

	baselineRepoURL      = "https://github.com/rezakaramad/kubepave"
	baselineRepoBranch   = "main"
	baselineRepoBasePath = "charts/baseline-tenant"

	gitopsRepoURL      = "https://github.com/rezakaramad/kubepave"
	gitopsRepoBranch   = "main"
	gitopsRepoBasePath = "charts/gitops-tenant"
)

// fatal marks the function response as fatally failed due to an internal error.
// It sets a FunctionSuccess=False condition with reason InternalError on both
// the composite resource and claim, then records the error as fatal to stop
// the composition pipeline.
func fatal(rsp *fnv1.RunFunctionResponse, err error, msg string) (*fnv1.RunFunctionResponse, error) {
	if err != nil {
		err = xperrors.Wrap(err, msg)
	} else {
		err = xperrors.New(msg)
	}
	response.ConditionFalse(rsp, "FunctionSuccess", "InternalError").
		WithMessage(err.Error()).
		TargetCompositeAndClaim()
	response.Fatal(rsp, err)
	return rsp, nil
}

// Function is the gRPC server that Crossplane calls to render tenant resources.
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger

	// Next-Insight client; nil when Next-Insight integration is not configured.
	nextInsight nextinsight.Client
}

func (f *Function) RunFunction(
	ctx context.Context,
	req *fnv1.RunFunctionRequest,
) (*fnv1.RunFunctionResponse, error) {
	log := f.log.WithValues("tag", req.GetMeta().GetTag())
	log.Info("Running function-xtenant-render")

	rsp := response.To(req, response.DefaultTTL)

	// ---------------------------------------------------------------------
	// 1. Load XR
	// ---------------------------------------------------------------------
	observedXR, err := request.GetObservedCompositeResource(req)
	if err != nil {
		return fatal(rsp, err, "cannot get observed composite resource")
	}
	if observedXR == nil || observedXR.Resource == nil || len(observedXR.Resource.UnstructuredContent()) == 0 {
		return fatal(rsp, nil, "missing observed composite resource")
	}

	// ---------------------------------------------------------------------
	// 2. Desired / observed state maps
	// ---------------------------------------------------------------------
	desired, err := request.GetDesiredComposedResources(req)
	if err != nil {
		return fatal(rsp, err, "cannot get desired composed resources")
	}

	observed, err := request.GetObservedComposedResources(req)
	if err != nil {
		return fatal(rsp, err, "cannot get observed composed resources")
	}

	// ---------------------------------------------------------------------
	// 3. Parse XR into XTenant
	// ---------------------------------------------------------------------
	var xd xtenant.XTenant
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		observedXR.Resource.UnstructuredContent(), &xd,
	); err != nil {
		return fatal(rsp, err, "cannot convert XR to XTenant")
	}

	if xd.Spec.DisplayName == "" {
		xd.Spec.DisplayName = xd.GetName()
	}

	// ---------------------------------------------------------------------
	// 4. Approval gate — do not render resources until approved
	// ---------------------------------------------------------------------
	if !xd.Spec.Approved {
		log.Info("XTenant not yet approved, skipping render", "tenant", xd.GetName())
		return rsp, nil
	}

	tenant := render.TenantSpec{
		XTenant:   xd,
		SyncRepos: []string{fmt.Sprintf("https://github.com/fluxdojo/platform-deploy-%s", xd.GetName())},
	}

	log = log.WithValues("tenant", tenant.GetName())

	// ---------------------------------------------------------------------
	// 5. Parse function input
	// ---------------------------------------------------------------------
	var input inputv1beta1.Input
	if err := request.GetInput(req, &input); err != nil {
		return fatal(rsp, err, "cannot parse function input")
	}

	bindings := input.Tenant.Bindings
	clusters := render.UniqueClustersFromBindings(bindings)
	log.Info("Resolved clusters", "clusters", clusters)
	log.Info("Resolved bindings", "bindings", bindings)

	// ---------------------------------------------------------------------
	// 6. Build Entra principal resources and resolve object IDs
	// ---------------------------------------------------------------------
	resolvedBindings := make([]render.ResolvedBinding, 0, len(bindings))
	waitingForPrincipal := false

	for _, binding := range bindings {
		maps.Copy(desired, render.BuildPrincipalResources(tenant, binding, input.Azure))

		objectID, ready := render.ResolveBindingPrincipalObjectID(observed, input.Azure, binding)
		if !ready {
			waitingForPrincipal = true
			continue
		}

		resolvedBindings = append(resolvedBindings, render.ResolvedBinding{
			Role:              binding.Name,
			Cluster:           binding.Cluster,
			EnvironmentPrefix: binding.EnvironmentPrefix,
			PrincipalObjectID: objectID,
		})
	}

	if waitingForPrincipal {
		delete(desired, "tenant-rendered-manifests")
		if err := response.SetDesiredComposedResources(rsp, desired); err != nil {
			return fatal(rsp, err, "cannot set desired composed resources")
		}
		response.ConditionFalse(rsp, conditionRendered, "WaitingForPrincipalObjectID").
			WithMessage(fmt.Sprintf("Waiting for principal object IDs for tenant %q", tenant.GetName())).
			TargetComposite()
		return rsp, nil
	}

	// ---------------------------------------------------------------------
	// 7. Render ArgoCD Applications
	// ---------------------------------------------------------------------
	baselineApps, err := render.BuildBaselineApplications(
		tenant, clusters,
		baselineRepoURL, baselineRepoBranch, baselineRepoBasePath,
	)
	if err != nil {
		return fatal(rsp, err, "cannot build baseline applications")
	}

	gitopsApp, err := render.BuildGitopsApplication(
		tenant, resolvedBindings, input.Azure,
		gitopsRepoURL, gitopsRepoBranch, gitopsRepoBasePath,
	)
	if err != nil {
		return fatal(rsp, err, "cannot build gitops application")
	}

	// ---------------------------------------------------------------------
	// 8. Enrich with Next-Insight metadata labels (optional)
	// ---------------------------------------------------------------------
	nextInsightLabels, err := render.FetchTenantLabels(ctx, f.nextInsight, tenant.Spec.TeamID, input.NextInsight.LabelPrefix)
	if err != nil {
		// Non-fatal: log and continue — metadata enrichment must not block provisioning.
		log.Info("Skipping Next-Insight label enrichment", "error", err)
		nextInsightLabels = map[string]string{}
	}

	render.ApplyNextInsightLabels(nextInsightLabels, append(baselineApps, gitopsApp)...)

	// ---------------------------------------------------------------------
	// 9. Bundle to YAML and write RepositoryFile
	// ---------------------------------------------------------------------
	resources := make([]*composed.Unstructured, 0, 1+len(baselineApps))
	resources = append(resources, gitopsApp)
	resources = append(resources, baselineApps...)

	content, err := render.BundleYAML(resources...)
	if err != nil {
		return fatal(rsp, err, "cannot bundle resources")
	}

	github := input.Github.WithDefaults()

	repoFile := render.BuildRepositoryFile(tenant, content, render.RepositoryFileConfig{
		ProviderConfigName: github.ProviderConfigName,
		Repository:         exportRepository,
		Branch:             exportRepoBranch,
		BasePath:           exportRepoBasePath,
		CommitAuthor:       github.CommitAuthor,
		CommitEmail:        github.CommitEmail,
	})

	desired["tenant-rendered-manifests"] = &resource.DesiredComposed{Resource: repoFile}

	if err := response.SetDesiredComposedResources(rsp, desired); err != nil {
		return fatal(rsp, err, "cannot set desired composed resources")
	}

	response.ConditionTrue(rsp, conditionRendered, "Available").
		WithMessage(fmt.Sprintf("Rendered %d resources for tenant %q", len(resources), tenant.GetName())).
		TargetComposite()

	return rsp, nil
}
