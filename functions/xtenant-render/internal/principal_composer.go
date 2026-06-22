package render

import (
	inputv1beta1 "github.com/rezakaramad/crosskit/functions/xtenant-render/input/v1beta1"
	runner "github.com/rezakaramad/crosskit/modules/runner"
	xtenant "github.com/rezakaramad/crosskit/types/xtenant"

	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
)

// tenantSpecFromCtx builds a TenantSpec from the runner Context, applying
// the DisplayName default when the XR does not set one explicitly.
func tenantSpecFromCtx(ctx runner.Context[*xtenant.XTenant, *inputv1beta1.Input]) TenantSpec {
	xr := *ctx.XR
	if xr.Spec.DisplayName == "" {
		xr.Spec.DisplayName = xr.GetName()
	}
	return TenantSpec{
		XTenant:   xr,
		SyncRepos: []string{"https://github.com/fluxdojo/platform-deploy-" + xr.GetName()},
	}
}

// ── PrincipalGroupComposer ────────────────────────────────────────────────────

// PrincipalGroupComposer manages one Entra group for a single binding.
// It does not set a condition on the XR (ConditionType returns "") —
// readiness is surfaced only through the RepositoryFileComposer.
type PrincipalGroupComposer struct {
	binding inputv1beta1.BindingInput
}

func NewPrincipalGroupComposer(binding inputv1beta1.BindingInput) *PrincipalGroupComposer {
	return &PrincipalGroupComposer{binding: binding}
}

func (c *PrincipalGroupComposer) ConditionType() string { return "" }

func (c *PrincipalGroupComposer) ResourceName(ctx runner.Context[*xtenant.XTenant, *inputv1beta1.Input]) resource.Name {
	return principalResourceName(ctx.Input.Azure, c.binding)
}

func (c *PrincipalGroupComposer) Compose(ctx runner.Context[*xtenant.XTenant, *inputv1beta1.Input]) (*composed.Unstructured, error) {
	return buildPrincipalGroup(tenantSpecFromCtx(ctx), c.binding), nil
}

func (c *PrincipalGroupComposer) IsReady(_ runner.Context[*xtenant.XTenant, *inputv1beta1.Input], observed *composed.Unstructured) bool {
	if observed == nil {
		return false
	}
	if id, err := observed.GetString("status.atProvider.objectId"); err == nil && id != "" {
		return true
	}
	id, err := observed.GetString("status.atProvider.id")
	return err == nil && id != ""
}

func (c *PrincipalGroupComposer) ConnectionDetails(_ runner.Context[*xtenant.XTenant, *inputv1beta1.Input], _ *composed.Unstructured) map[string]string {
	return nil
}

// ── PrincipalUserComposer ─────────────────────────────────────────────────────

// PrincipalUserComposer manages one Entra user for a single binding.
type PrincipalUserComposer struct {
	binding inputv1beta1.BindingInput
}

func NewPrincipalUserComposer(binding inputv1beta1.BindingInput) *PrincipalUserComposer {
	return &PrincipalUserComposer{binding: binding}
}

func (c *PrincipalUserComposer) ConditionType() string { return "" }

func (c *PrincipalUserComposer) ResourceName(ctx runner.Context[*xtenant.XTenant, *inputv1beta1.Input]) resource.Name {
	return principalResourceName(ctx.Input.Azure, c.binding)
}

func (c *PrincipalUserComposer) Compose(ctx runner.Context[*xtenant.XTenant, *inputv1beta1.Input]) (*composed.Unstructured, error) {
	return buildPrincipalUser(tenantSpecFromCtx(ctx), c.binding, ctx.Input.Azure), nil
}

func (c *PrincipalUserComposer) IsReady(_ runner.Context[*xtenant.XTenant, *inputv1beta1.Input], observed *composed.Unstructured) bool {
	if observed == nil {
		return false
	}
	if id, err := observed.GetString("status.atProvider.objectId"); err == nil && id != "" {
		return true
	}
	id, err := observed.GetString("status.atProvider.id")
	return err == nil && id != ""
}

func (c *PrincipalUserComposer) ConnectionDetails(_ runner.Context[*xtenant.XTenant, *inputv1beta1.Input], _ *composed.Unstructured) map[string]string {
	return nil
}

// ── PrincipalPasswordComposer ─────────────────────────────────────────────────

// PrincipalPasswordComposer manages the ESO Password generator for a user binding.
type PrincipalPasswordComposer struct {
	binding inputv1beta1.BindingInput
}

func NewPrincipalPasswordComposer(binding inputv1beta1.BindingInput) *PrincipalPasswordComposer {
	return &PrincipalPasswordComposer{binding: binding}
}

func (c *PrincipalPasswordComposer) ConditionType() string { return "" }

func (c *PrincipalPasswordComposer) ResourceName(_ runner.Context[*xtenant.XTenant, *inputv1beta1.Input]) resource.Name {
	return userPasswordResourceName(c.binding)
}

func (c *PrincipalPasswordComposer) Compose(ctx runner.Context[*xtenant.XTenant, *inputv1beta1.Input]) (*composed.Unstructured, error) {
	secretName, _ := userResourceNames(tenantSpecFromCtx(ctx), c.binding)
	return buildPrincipalUserPassword(c.binding, secretName), nil
}

func (c *PrincipalPasswordComposer) IsReady(_ runner.Context[*xtenant.XTenant, *inputv1beta1.Input], _ *composed.Unstructured) bool {
	return true
}

func (c *PrincipalPasswordComposer) ConnectionDetails(_ runner.Context[*xtenant.XTenant, *inputv1beta1.Input], _ *composed.Unstructured) map[string]string {
	return nil
}

// ── PrincipalPasswordSecretComposer ──────────────────────────────────────────

// PrincipalPasswordSecretComposer manages the ESO ExternalSecret for a user binding.
type PrincipalPasswordSecretComposer struct {
	binding inputv1beta1.BindingInput
}

func NewPrincipalPasswordSecretComposer(binding inputv1beta1.BindingInput) *PrincipalPasswordSecretComposer {
	return &PrincipalPasswordSecretComposer{binding: binding}
}

func (c *PrincipalPasswordSecretComposer) ConditionType() string { return "" }

func (c *PrincipalPasswordSecretComposer) ResourceName(_ runner.Context[*xtenant.XTenant, *inputv1beta1.Input]) resource.Name {
	return userPasswordSecretResourceName(c.binding)
}

func (c *PrincipalPasswordSecretComposer) Compose(ctx runner.Context[*xtenant.XTenant, *inputv1beta1.Input]) (*composed.Unstructured, error) {
	secretName, _ := userResourceNames(tenantSpecFromCtx(ctx), c.binding)
	return buildPrincipalUserPasswordSecret(c.binding, secretName), nil
}

func (c *PrincipalPasswordSecretComposer) IsReady(_ runner.Context[*xtenant.XTenant, *inputv1beta1.Input], _ *composed.Unstructured) bool {
	return true
}

func (c *PrincipalPasswordSecretComposer) ConnectionDetails(_ runner.Context[*xtenant.XTenant, *inputv1beta1.Input], _ *composed.Unstructured) map[string]string {
	return nil
}
