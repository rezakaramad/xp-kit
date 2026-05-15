package validate

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	inputv1beta1 "github.com/rezakaramad/crossplane-toolkit/functions/xtenant-validate/input/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// DNSClient checks whether a fully qualified DNS name is available.
type DNSClient interface {
	CheckDNSAvailable(ctx context.Context, fqdn string) (DNSAvailabilityResult, error)
}

// DNSAvailabilityResult represents the availability of a DNS name.
type DNSAvailabilityResult struct {
	Available bool
	Reason    string
}

// BuildDNSClient constructs the right DNSClient implementation based on the
// provider field in the function input.
//
// It is called on every RunFunction invocation, so any Secret rotation is
// picked up automatically without restarting the pod.
func BuildDNSClient(ctx context.Context, cfg inputv1beta1.DNSInput, kube ctrlclient.Client) (DNSClient, error) {
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
