package main

import (
	"context"
	"fmt"
	"maps"

	xperrors "github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	inputv1beta1 "github.com/rezakaramad/crossplane-toolkit/functions/xtenant-render/input/v1beta1"
	xtenant "github.com/rezakaramad/crossplane-toolkit/types/xtenant"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/crossplane/function-sdk-go/response"
)

func uniqueClustersFromBindings(bindings []inputv1beta1.BindingInput) []xtenant.Cluster {
	clusters := make([]xtenant.Cluster, 0, len(bindings))
	seen := make(map[string]struct{}, len(bindings))

	for _, binding := range bindings {
		key := binding.Cluster + "/" + binding.EnvironmentPrefix
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		clusters = append(clusters, xtenant.Cluster{Name: binding.Cluster, Prefix: binding.EnvironmentPrefix})
	}

	return clusters
}

// Function is the gRPC server that Crossplane calls to render tenant resources.
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger

	// Git export (final bundle destination)
	exportRepository   string
	exportRepoBranch   string
	exportRepoBasePath string

	// Crossplane namespace
	crossplaneNamespace string

	// Baseline Application source (ArgoCD)
	baselineRepoURL      string
	baselineRepoBranch   string
	baselineRepoBasePath string

	// GitOps Application source (ArgoCD)
	gitopsRepoURL      string
	gitopsRepoBranch   string
	gitopsRepoBasePath string
}

func (f *Function) RunFunction(
	_ context.Context,
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
		response.Fatal(rsp, xperrors.Wrap(err, "cannot get observed composite resource"))
		return rsp, nil
	}
	if observedXR == nil || observedXR.Resource == nil || len(observedXR.Resource.UnstructuredContent()) == 0 {
		response.Fatal(rsp, xperrors.New("missing observed composite resource"))
		return rsp, nil
	}

	// ---------------------------------------------------------------------
	// 2. Desired / observed state maps
	// ---------------------------------------------------------------------
	desired, err := request.GetDesiredComposedResources(req)
	if err != nil {
		response.Fatal(rsp, xperrors.Wrap(err, "cannot get desired composed resources"))
		return rsp, nil
	}

	observed, err := request.GetObservedComposedResources(req)
	if err != nil {
		response.Fatal(rsp, xperrors.Wrap(err, "cannot get observed composed resources"))
		return rsp, nil
	}

	// ---------------------------------------------------------------------
	// 3. Parse XR into XTenant
	// ---------------------------------------------------------------------
	var xd xtenant.XTenant
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		observedXR.Resource.UnstructuredContent(), &xd,
	); err != nil {
		response.Fatal(rsp, xperrors.Wrap(err, "cannot convert XR to XTenant"))
		return rsp, nil
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

	tenant := TenantSpec{
		XTenant:   xd,
		SyncRepos: []string{fmt.Sprintf("https://github.com/fluxdojo/platform-deploy-%s", xd.GetName())},
	}

	log = log.WithValues("tenant", tenant.GetName())

	// ---------------------------------------------------------------------
	// 5. Parse function input
	// ---------------------------------------------------------------------
	var input inputv1beta1.Input
	if err := request.GetInput(req, &input); err != nil {
		response.Fatal(rsp, xperrors.Wrap(err, "cannot parse function input"))
		return rsp, nil
	}

	bindings := input.Tenant.Bindings
	clusters := uniqueClustersFromBindings(bindings)
	log.Info("Resolved clusters", "clusters", clusters)
	log.Info("Resolved bindings", "bindings", bindings)

	// ---------------------------------------------------------------------
	// 6. Build Entra principal resources and resolve object IDs
	// ---------------------------------------------------------------------
	resolvedBindings := make([]ResolvedBinding, 0, len(bindings))
	waitingForPrincipal := false

	for _, binding := range bindings {
		maps.Copy(desired, buildPrincipalResources(tenant, binding, input.Azure))

		objectID, ready := resolveBindingPrincipalObjectID(observed, input.Azure, binding)
		if !ready {
			waitingForPrincipal = true
			continue
		}

		resolvedBindings = append(resolvedBindings, ResolvedBinding{
			Role:              binding.Name,
			Cluster:           binding.Cluster,
			EnvironmentPrefix: binding.EnvironmentPrefix,
			PrincipalObjectID: objectID,
		})
	}

	if waitingForPrincipal {
		delete(desired, "tenant-rendered-manifests")
		if err := response.SetDesiredComposedResources(rsp, desired); err != nil {
			response.Fatal(rsp, xperrors.Wrap(err, "cannot set desired composed resources"))
			return rsp, nil
		}
		response.ConditionFalse(rsp, "Rendered", "WaitingForPrincipalObjectID").
			WithMessage(fmt.Sprintf("Waiting for principal object IDs for tenant %q", tenant.GetName())).
			TargetComposite()
		return rsp, nil
	}

	// ---------------------------------------------------------------------
	// 7. Render ArgoCD Applications
	// ---------------------------------------------------------------------
	baselineApps, err := buildBaselineApplications(
		tenant, clusters,
		f.baselineRepoURL, f.baselineRepoBranch, f.baselineRepoBasePath,
	)
	if err != nil {
		response.Fatal(rsp, xperrors.Wrap(err, "cannot build baseline applications"))
		return rsp, nil
	}

	gitopsApp, err := buildGitopsApplication(
		tenant, resolvedBindings, input.Azure,
		f.gitopsRepoURL, f.gitopsRepoBranch, f.gitopsRepoBasePath,
	)
	if err != nil {
		response.Fatal(rsp, xperrors.Wrap(err, "cannot build gitops application"))
		return rsp, nil
	}

	// ---------------------------------------------------------------------
	// 8. Bundle to YAML and write RepositoryFile
	// ---------------------------------------------------------------------
	resources := make([]*composed.Unstructured, 0, 1+len(baselineApps))
	resources = append(resources, gitopsApp)
	resources = append(resources, baselineApps...)

	content, err := bundleYAML(resources...)
	if err != nil {
		response.Fatal(rsp, xperrors.Wrap(err, "cannot bundle resources"))
		return rsp, nil
	}

	providerConfigName := input.Github.ProviderConfigName
	if providerConfigName == "" {
		providerConfigName = "github-rezakaramad"
	}
	commitAuthor := input.Github.CommitAuthor
	if commitAuthor == "" {
		commitAuthor = "Crossplane"
	}
	commitEmail := input.Github.CommitEmail
	if commitEmail == "" {
		commitEmail = "crossplane@rezakara.demo"
	}

	repoFile := buildRepositoryFile(tenant, content, RepositoryFileConfig{
		Namespace:          f.crossplaneNamespace,
		ProviderConfigName: providerConfigName,
		Repository:         f.exportRepository,
		Branch:             f.exportRepoBranch,
		BasePath:           f.exportRepoBasePath,
		CommitAuthor:       commitAuthor,
		CommitEmail:        commitEmail,
	})

	desired["tenant-rendered-manifests"] = &resource.DesiredComposed{Resource: repoFile}

	if err := response.SetDesiredComposedResources(rsp, desired); err != nil {
		response.Fatal(rsp, xperrors.Wrap(err, "cannot set desired composed resources"))
		return rsp, nil
	}

	response.ConditionTrue(rsp, "Rendered", "Available").
		WithMessage(fmt.Sprintf("Rendered %d resources for tenant %q", len(resources), tenant.GetName())).
		TargetComposite()

	return rsp, nil
}
