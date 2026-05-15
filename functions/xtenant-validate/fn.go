package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	inputv1beta1 "github.com/rezakaramad/crossplane-toolkit/functions/xtenant-validate/input/v1beta1"
	validate "github.com/rezakaramad/crossplane-toolkit/functions/xtenant-validate/internal"
	"github.com/rezakaramad/crossplane-toolkit/modules/nextinsight"
	xtenant "github.com/rezakaramad/crossplane-toolkit/types/xtenant"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/client/config"

	xperrors "github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
)

const (
	conditionValid    = "Valid"
	conditionApproved = "Approved"
	conditionReady    = "Ready"
)

// Function is the gRPC server that Crossplane calls to run validation.
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	log         logging.Logger
	kube        ctrlclient.Client
	dns         validate.DNSClient
	nextInsight nextinsight.Client
}

// newFunction builds the Function with all external clients initialised.
func newFunction(log logging.Logger) (*Function, error) {
	cfg, err := ctrlconfig.GetConfig()
	if err != nil {
		return nil, err
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	kubeClient, err := ctrlclient.New(cfg, ctrlclient.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}

	return &Function{
		log:         log,
		kube:        kubeClient,
		nextInsight: newNextInsightClient(),
	}, nil
}

// newNextInsightClient returns a configured Next-Insight client when
// NEXTINSIGHT_BASE_URL is set, or nil to skip Next-Insight validation.
func newNextInsightClient() nextinsight.Client {
	baseURL := os.Getenv("NEXTINSIGHT_BASE_URL")
	if baseURL == "" {
		return nil
	}
	return nextinsight.New(baseURL, os.Getenv("NEXTINSIGHT_TOKEN"))
}

func (f *Function) RunFunction(
	ctx context.Context,
	req *fnv1.RunFunctionRequest,
) (*fnv1.RunFunctionResponse, error) {
	start := time.Now()

	log := f.log.WithValues("tag", req.GetMeta().GetTag())
	log.Info("Running function-xtenant-validate")

	rsp := response.To(req, response.DefaultTTL)

	// ---------------------------------------------------------------------
	// 1. Load XR
	// ---------------------------------------------------------------------
	xr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		return fatal(rsp, err, "cannot get observed XR")
	}

	// ---------------------------------------------------------------------
	// 2. Parse XR into XTenant
	// ---------------------------------------------------------------------
	tenantRequest, err := fromObservedXR(xr)
	if err != nil {
		return fail(rsp, xr, err, "cannot parse XR")
	}

	log = log.WithValues("tenant", tenantRequest.GetName())

	// ---------------------------------------------------------------------
	// 3. Parse function input
	// ---------------------------------------------------------------------
	var input inputv1beta1.Input
	if err := request.GetInput(req, &input); err != nil {
		return fail(rsp, xr, err, "cannot parse function input")
	}
	if input.DNS.ReferenceEnvironmentDomain == "" {
		return fail(rsp, xr, xperrors.New("dns.referenceEnvironmentDomain is required"), "cannot parse function input")
	}

	// ---------------------------------------------------------------------
	// 4. Approved tenants already passed validation before approval was set.
	// Skip external re-validation on subsequent reconciles.
	// ---------------------------------------------------------------------
	if validate.IsApproved(tenantRequest) {
		response.ConditionTrue(rsp, conditionValid, "ValidationPassed").TargetComposite()
		response.ConditionTrue(rsp, conditionApproved, "Approved").TargetComposite()

		validate.SetPhase(xr, xtenant.PhaseProvisioning)
		response.ConditionTrue(rsp, conditionReady, "Provisioning").
			WithMessage("XTenant approved, provisioning in progress").
			TargetComposite()

		log.Info("Skipping validation for approved tenant")
		return done(rsp, xr)
	}

	// ---------------------------------------------------------------------
	// 5. Resolve DNS client from input (or use injected override for tests)
	// ---------------------------------------------------------------------
	dnsClient := f.dns
	if dnsClient == nil {
		dnsClient, err = validate.BuildDNSClient(ctx, input.DNS, f.kube)
		if err != nil {
			return fail(rsp, xr, err, "cannot build dns client")
		}
	}

	// ---------------------------------------------------------------------
	// 6. Validation
	// ---------------------------------------------------------------------
	validate.SetPhase(xr, xtenant.PhaseValidating)

	if verr := validate.Validate(ctx, tenantRequest, validate.Deps{
		Kube:                       f.kube,
		DNS:                        dnsClient,
		ReferenceEnvironmentDomain: input.DNS.ReferenceEnvironmentDomain,
		NextInsight:                f.nextInsight,
	}); verr != nil {
		if verr.Retryable {
			validate.SetPhase(xr, xtenant.PhaseValidating)
		} else {
			validate.SetPhase(xr, xtenant.PhaseFailed)
		}

		response.ConditionFalse(rsp, conditionValid, verr.Reason).
			WithMessage(verr.Message).
			TargetComposite()

		response.ConditionFalse(rsp, conditionReady, "ValidationFailed").
			WithMessage("XTenant is not valid").
			TargetComposite()

		log.Info("Validation failed", "reason", verr.Reason)
		return done(rsp, xr)
	}

	response.ConditionTrue(rsp, conditionValid, "ValidationPassed").TargetComposite()

	// ---------------------------------------------------------------------
	// 7. Approval gate
	// ---------------------------------------------------------------------
	if !validate.IsApproved(tenantRequest) {
		validate.SetPhase(xr, xtenant.PhasePendingApproval)

		response.ConditionFalse(rsp, conditionApproved, "WaitingForApproval").TargetComposite()
		response.ConditionFalse(rsp, conditionReady, "WaitingForApproval").TargetComposite()

		log.Info("Waiting for approval")
		return done(rsp, xr)
	}

	response.ConditionTrue(rsp, conditionApproved, "Approved").TargetComposite()

	// ---------------------------------------------------------------------
	// 8. Approved — hand off to the renderer pipeline step
	// ---------------------------------------------------------------------
	validate.SetPhase(xr, xtenant.PhaseProvisioning)

	response.ConditionTrue(rsp, conditionReady, "Provisioning").
		WithMessage("XTenant approved, provisioning in progress").
		TargetComposite()

	log.Info("Reconciliation finished",
		"tenant", tenantRequest.GetName(),
		"duration", time.Since(start),
	)

	return done(rsp, xr)
}

// fromObservedXR converts the unstructured observed XR into a typed XTenant
// and validates that all required fields are present.
func fromObservedXR(xr *resource.Composite) (xtenant.XTenant, error) {
	var t xtenant.XTenant
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		xr.Resource.UnstructuredContent(), &t,
	); err != nil {
		return xtenant.XTenant{}, fmt.Errorf("cannot convert XR to XTenant: %w", err)
	}

	if t.GetName() == "" {
		return t, fmt.Errorf("metadata.name is required")
	}
	if strings.TrimSpace(t.Spec.DNSName) == "" {
		return t, fmt.Errorf("required field missing: spec.dnsName")
	}
	return t, nil
}

// ---------------------------------------------------------------------
// Response helpers
// ---------------------------------------------------------------------

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

func fail(rsp *fnv1.RunFunctionResponse, xr *resource.Composite, err error, msg string) (*fnv1.RunFunctionResponse, error) {
	validate.SetPhase(xr, xtenant.PhaseFailed)
	response.Fatal(rsp, xperrors.Wrap(err, msg))
	return done(rsp, xr)
}

func done(rsp *fnv1.RunFunctionResponse, xr *resource.Composite) (*fnv1.RunFunctionResponse, error) {
	xr.Resource.SetManagedFields(nil)
	_ = response.SetDesiredCompositeResource(rsp, xr)
	return rsp, nil
}
