package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	xperrors "github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	inputv1beta1 "github.com/rezakaramad/crossplane-toolkit/functions/xtenant-validate/input/v1beta1"
	xtenant "github.com/rezakaramad/crossplane-toolkit/types/xtenant"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
)

// Function is the gRPC server that Crossplane calls to run validation.
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	log  logging.Logger
	kube ctrlclient.Client
	dns  DNSClient
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
	if input.DNS.BaseDomain == "" {
		return fail(rsp, xr, xperrors.New("dns.baseDomain is required"), "cannot parse function input")
	}
	if len(input.Clusters) == 0 {
		return fail(rsp, xr, xperrors.New("clusters is required"), "cannot parse function input")
	}

	workloadClusters := make([]xtenant.Cluster, 0, len(input.Clusters))
	for _, cluster := range input.Clusters {
		if cluster.Name == "" || cluster.Prefix == "" {
			return fail(rsp, xr, xperrors.New("clusters entries require name and prefix"), "cannot parse function input")
		}
		workloadClusters = append(workloadClusters, xtenant.Cluster{
			Name:   cluster.Name,
			Prefix: cluster.Prefix,
		})
	}

	// ---------------------------------------------------------------------
	// 4. Approved tenants already passed validation before approval was set.
	// Skip external re-validation on subsequent reconciles.
	// ---------------------------------------------------------------------
	if IsApproved(tenantRequest) {
		response.ConditionTrue(rsp, "Valid", "ValidationPassed").TargetComposite()
		response.ConditionTrue(rsp, "Approved", "Approved").TargetComposite()

		SetPhase(xr, PhaseProvisioning)
		response.ConditionTrue(rsp, "Ready", "Provisioning").
			WithMessage("XTenant approved, provisioning in progress").
			TargetComposite()

		log.Info("Skipping validation for approved tenant")
		return done(rsp, xr)
	}

	// ---------------------------------------------------------------------
	// 4. Resolve DNS client from input (or use injected override for tests)
	// ---------------------------------------------------------------------
	dnsClient := f.dns
	if dnsClient == nil {
		dnsClient, err = buildDNSClient(ctx, input.DNS, f.kube)
		if err != nil {
			return fail(rsp, xr, err, "cannot build dns client")
		}
	}

	// ---------------------------------------------------------------------
	// 5. Validation
	// ---------------------------------------------------------------------
	SetPhase(xr, PhaseValidating)

	if verr := Validate(ctx, tenantRequest, Deps{
		Kube:             f.kube,
		DNS:              dnsClient,
		BaseDomain:       input.DNS.BaseDomain,
		WorkloadClusters: workloadClusters,
	}); verr != nil {
		if verr.Retryable {
			SetPhase(xr, PhaseValidating)
		} else {
			SetPhase(xr, PhaseFailed)
		}

		response.ConditionFalse(rsp, "Valid", verr.Reason).
			WithMessage(verr.Message).
			TargetComposite()

		response.ConditionFalse(rsp, "Ready", "ValidationFailed").
			WithMessage("XTenant is not valid").
			TargetComposite()

		log.Info("Validation failed", "reason", verr.Reason)
		return done(rsp, xr)
	}

	response.ConditionTrue(rsp, "Valid", "ValidationPassed").TargetComposite()

	// ---------------------------------------------------------------------
	// 6. Approval gate
	// ---------------------------------------------------------------------
	if !IsApproved(tenantRequest) {
		SetPhase(xr, PhasePendingApproval)

		response.ConditionFalse(rsp, "Approved", "WaitingForApproval").TargetComposite()
		response.ConditionFalse(rsp, "Ready", "WaitingForApproval").TargetComposite()

		log.Info("Waiting for approval")
		return done(rsp, xr)
	}

	response.ConditionTrue(rsp, "Approved", "Approved").TargetComposite()

	// ---------------------------------------------------------------------
	// 7. Approved — hand off to the renderer pipeline step
	// ---------------------------------------------------------------------
	SetPhase(xr, PhaseProvisioning)

	response.ConditionTrue(rsp, "Ready", "Provisioning").
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
	if strings.TrimSpace(t.Spec.Owner.Team) == "" {
		return t, fmt.Errorf("required field missing: spec.owner.team")
	}

	return t, nil
}

// ---------------------------------------------------------------------
// Response helpers
// ---------------------------------------------------------------------

func fatal(rsp *fnv1.RunFunctionResponse, err error, msg string) (*fnv1.RunFunctionResponse, error) {
	response.Fatal(rsp, xperrors.Wrap(err, msg))
	return rsp, nil
}

func fail(rsp *fnv1.RunFunctionResponse, xr *resource.Composite, err error, msg string) (*fnv1.RunFunctionResponse, error) {
	SetPhase(xr, PhaseFailed)
	response.Fatal(rsp, xperrors.Wrap(err, msg))
	return done(rsp, xr)
}

func done(rsp *fnv1.RunFunctionResponse, xr *resource.Composite) (*fnv1.RunFunctionResponse, error) {
	xr.Resource.SetManagedFields(nil)
	_ = response.SetDesiredCompositeResource(rsp, xr)
	return rsp, nil
}

// buildDNSClient constructs the right DNSClient implementation based on the
// provider field in the function input.
//
// It is called on every RunFunction invocation, so any Secret rotation is
// picked up automatically without restarting the pod.
func buildDNSClient(ctx context.Context, cfg inputv1beta1.DNSInput, kube ctrlclient.Client) (DNSClient, error) {
	switch cfg.Provider {
	case "clouddns":
		if cfg.GCPProject == "" {
			return nil, fmt.Errorf("dns.gcpProject is required when provider is clouddns")
		}
		// Workload Identity supplies credentials automatically — no API key needed.
		return NewGCPDNSClient(ctx, cfg.GCPProject)

	case "powerdns", "":
		if cfg.APIURL == "" {
			return nil, fmt.Errorf("dns.apiURL is required when provider is powerdns")
		}
		if cfg.CredentialsSecretRef == nil {
			return nil, fmt.Errorf("dns.credentialsSecretRef is required when provider is powerdns")
		}
		apiKey, err := readSecretKey(ctx, kube, cfg.CredentialsSecretRef)
		if err != nil {
			return nil, fmt.Errorf("cannot read powerdns credentials: %w", err)
		}
		return NewPowerDNSClient(
			cfg.APIURL,
			apiKey,
			&http.Client{Timeout: 5 * time.Second},
		), nil

	default:
		return nil, fmt.Errorf("unknown dns.provider %q: supported values are powerdns, clouddns", cfg.Provider)
	}
}

// readSecretKey reads a single key from a Kubernetes Secret.
// Because this is called on every reconcile, rotation is picked up
// within one reconcile loop after the Secret is updated.
func readSecretKey(ctx context.Context, kube ctrlclient.Client, ref *inputv1beta1.SecretKeyRef) (string, error) {
	secret := &corev1.Secret{}
	if err := kube.Get(ctx, types.NamespacedName{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}, secret); err != nil {
		return "", fmt.Errorf("get secret %s/%s: %w", ref.Namespace, ref.Name, err)
	}
	val, ok := secret.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret %s/%s", ref.Key, ref.Namespace, ref.Name)
	}
	return strings.TrimSpace(string(val)), nil
}
