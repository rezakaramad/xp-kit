package main

import (
	"context"

	inputv1beta1 "github.com/rezakaramad/crosskit/functions/xtenant-render/input/v1beta1"
	render "github.com/rezakaramad/crosskit/functions/xtenant-render/internal"
	"github.com/rezakaramad/crosskit/modules/nextinsight"
	runner "github.com/rezakaramad/crosskit/modules/runner"
	xtenant "github.com/rezakaramad/crosskit/types/xtenant"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/response"
)

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

	// ── Approval gate ────────────────────────────────────────────────────────
	// Check before setting up the runner — the whole function is a no-op until
	// the tenant is approved by a human operator.
	observedXR, err := request.GetObservedCompositeResource(req)
	if err != nil {
		return render.Fatal(rsp, err, "cannot get observed composite resource")
	}
	if observedXR == nil || observedXR.Resource == nil || len(observedXR.Resource.UnstructuredContent()) == 0 {
		return render.Fatal(rsp, nil, "missing observed composite resource")
	}

	var xr xtenant.XTenant
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		observedXR.Resource.UnstructuredContent(), &xr,
	); err != nil {
		return render.Fatal(rsp, err, "cannot decode XR to XTenant")
	}

	if !xr.Spec.Approved {
		log.Info("XTenant not yet approved, skipping render", "tenant", xr.GetName())
		return rsp, nil
	}

	// ── Dynamic composer registration ────────────────────────────────────────
	// Parse the input now so we can register one principal composer per binding.
	// The runner will re-decode input internally during Run — the small overhead
	// is acceptable compared to passing bindings through an alternative API.
	var input inputv1beta1.Input
	if err := request.GetInput(req, &input); err != nil {
		return render.Fatal(rsp, err, "cannot parse function input")
	}

	r := runner.New[*xtenant.XTenant, *inputv1beta1.Input](req, log)

	for _, binding := range input.Tenant.Bindings {
		if input.Azure.PrincipalType == "user" {
			runner.Register(r, render.NewPrincipalPasswordComposer(binding))
			runner.Register(r, render.NewPrincipalPasswordSecretComposer(binding))
			runner.Register(r, render.NewPrincipalUserComposer(binding))
		} else {
			runner.Register(r, render.NewPrincipalGroupComposer(binding))
		}
	}

	runner.Register(r, render.NewRepositoryFileComposer(f.nextInsight))

	return r.Run(ctx)
}
