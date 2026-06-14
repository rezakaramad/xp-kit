package render

import (
	"fmt"

	inputv1beta1 "github.com/rezakaramad/crossplane-toolkit/functions/xtenant-render/input/v1beta1"
	groupsv1beta1 "github.com/upbound/provider-azuread/v2/apis/namespaced/groups/v1beta1"
	usersv1beta1 "github.com/upbound/provider-azuread/v2/apis/namespaced/users/v1beta1"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	commonv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	commonv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
)

const (
	defaultUserPrincipalDomain = "rkaramadgmail.onmicrosoft.com"
	principalTypeUser          = "user"
)

// ResolvedBinding holds a fully resolved role-cluster binding including the
// Entra principal object ID that was provisioned for it.
type ResolvedBinding struct {
	Role              string
	Cluster           string
	EnvironmentPrefix string
	PrincipalObjectID string
}

func usesUserPrincipal(azure inputv1beta1.AzureInput) bool {
	return azure.PrincipalType == principalTypeUser
}

func principalResourceName(azure inputv1beta1.AzureInput, binding inputv1beta1.BindingInput) resource.Name {
	if usesUserPrincipal(azure) {
		return resource.Name(fmt.Sprintf("entra-user-%s", binding.Name))
	}
	return resource.Name(fmt.Sprintf("entra-group-%s-%s-%s", binding.Name, binding.Cluster, binding.EnvironmentPrefix))
}

func userPasswordResourceName(binding inputv1beta1.BindingInput) resource.Name {
	return resource.Name(fmt.Sprintf("entra-user-password-%s", binding.Name))
}

func userPasswordSecretResourceName(binding inputv1beta1.BindingInput) resource.Name {
	return resource.Name(fmt.Sprintf("entra-user-password-secret-%s", binding.Name))
}

func userResourceNames(t TenantSpec, binding inputv1beta1.BindingInput) (secretName, principalName string) {
	principalName = fmt.Sprintf("%s-%s", t.GetName(), binding.Name)
	secretName = fmt.Sprintf("%s-initial-password", principalName)
	return secretName, principalName
}

func buildPrincipalGroup(t TenantSpec, binding inputv1beta1.BindingInput) *composed.Unstructured {
	displayName := fmt.Sprintf("%s-%s-%s",
		t.Spec.DisplayName,
		cases.Title(language.English).String(binding.Name),
		cases.Title(language.English).String(binding.EnvironmentPrefix),
	)
	securityEnabled := true

	group := &groupsv1beta1.Group{
		TypeMeta: metav1.TypeMeta{
			APIVersion: groupsv1beta1.CRDGroupVersion.String(),
			Kind:       groupsv1beta1.Group_Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s-%s", t.GetName(), binding.Name, binding.EnvironmentPrefix),
			Labels: map[string]string{
				"app.kubernetes.io/managed-by":  managedByCrossplane,
				"platform.rezakara.demo/tenant": t.GetName(),
				"platform.rezakara.demo/role":   binding.Name,
				"platform.rezakara.demo/prefix": binding.EnvironmentPrefix,
			},
		},
		Spec: groupsv1beta1.GroupSpec{
			ForProvider: groupsv1beta1.GroupParameters{
				DisplayName:     &displayName,
				SecurityEnabled: &securityEnabled,
			},
			ManagedResourceSpec: commonv2.ManagedResourceSpec{
				ProviderConfigReference: &commonv1.ProviderConfigReference{
					Name: "azuread",
					Kind: "ProviderConfig",
				},
			},
		},
	}
	return toComposed(group)
}

func buildPrincipalUserPassword(_ inputv1beta1.BindingInput, secretName string) *composed.Unstructured {
	password := composed.New()
	password.SetAPIVersion("generators.external-secrets.io/v1alpha1")
	password.SetKind("Password")
	password.SetName(secretName)
	_ = password.SetValue("spec.length", int64(32))
	return password
}

func buildPrincipalUserPasswordSecret(_ inputv1beta1.BindingInput, secretName string) *composed.Unstructured {
	externalSecret := composed.New()
	externalSecret.SetAPIVersion("external-secrets.io/v1")
	externalSecret.SetKind("ExternalSecret")
	externalSecret.SetName(secretName)
	_ = externalSecret.SetValue("spec.target.name", secretName)
	_ = externalSecret.SetValue("spec.dataFrom", []any{
		map[string]any{
			"sourceRef": map[string]any{
				"generatorRef": map[string]any{
					"kind":          "Password",
					metadataNameKey: secretName,
				},
			},
		},
	})
	return externalSecret
}

func buildPrincipalUser(t TenantSpec, binding inputv1beta1.BindingInput, azure inputv1beta1.AzureInput) *composed.Unstructured {
	secretName, principalName := userResourceNames(t, binding)
	domain := azure.UserPrincipalDomain
	if domain == "" {
		domain = defaultUserPrincipalDomain
	}

	upn := fmt.Sprintf("%s@%s", principalName, domain)
	displayName := fmt.Sprintf("%s %s",
		t.Spec.DisplayName,
		cases.Title(language.English).String(binding.Name),
	)

	user := &usersv1beta1.User{
		TypeMeta: metav1.TypeMeta{
			APIVersion: usersv1beta1.CRDGroupVersion.String(),
			Kind:       usersv1beta1.User_Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: principalName,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by":          managedByCrossplane,
				"platform.rezakara.demo/tenant":         t.GetName(),
				"platform.rezakara.demo/role":           binding.Name,
				"platform.rezakara.demo/principal-type": principalTypeUser,
			},
		},
		Spec: usersv1beta1.UserSpec{
			ForProvider: usersv1beta1.UserParameters{
				UserPrincipalName: &upn,
				DisplayName:       &displayName,
			},
			InitProvider: usersv1beta1.UserInitParameters{
				PasswordSecretRef: &commonv1.LocalSecretKeySelector{
					LocalSecretReference: commonv1.LocalSecretReference{Name: secretName},
					Key:                  "password",
				},
			},
			ManagedResourceSpec: commonv2.ManagedResourceSpec{
				ProviderConfigReference: &commonv1.ProviderConfigReference{
					Name: "azuread",
					Kind: "ProviderConfig",
				},
			},
		},
	}
	return toComposed(user)
}

// BuildPrincipalResources returns the desired composed resources for the tenant's Entra principal.
func BuildPrincipalResources(t TenantSpec, binding inputv1beta1.BindingInput, azure inputv1beta1.AzureInput) map[resource.Name]*resource.DesiredComposed {
	if !usesUserPrincipal(azure) {
		return map[resource.Name]*resource.DesiredComposed{
			principalResourceName(azure, binding): {Resource: buildPrincipalGroup(t, binding)},
		}
	}

	secretName, _ := userResourceNames(t, binding)
	return map[resource.Name]*resource.DesiredComposed{
		userPasswordResourceName(binding):       {Resource: buildPrincipalUserPassword(binding, secretName)},
		userPasswordSecretResourceName(binding): {Resource: buildPrincipalUserPasswordSecret(binding, secretName)},
		principalResourceName(azure, binding):   {Resource: buildPrincipalUser(t, binding, azure)},
	}
}

// ResolveBindingPrincipalObjectID looks up the Entra object ID from the observed composed resource.
func ResolveBindingPrincipalObjectID(observed map[resource.Name]resource.ObservedComposed, azure inputv1beta1.AzureInput, binding inputv1beta1.BindingInput) (string, bool) {
	observedResource, ok := observed[principalResourceName(azure, binding)]
	if !ok || observedResource.Resource == nil {
		return "", false
	}

	objectID, err := observedResource.Resource.GetString("status.atProvider.objectId")
	if err == nil && objectID != "" {
		return objectID, true
	}

	providerID, idErr := observedResource.Resource.GetString("status.atProvider.id")
	if idErr == nil && providerID != "" {
		return providerID, true
	}

	// Neither field is populated yet — provider hasn't synced the resource.
	// Signal "not ready" so the caller waits rather than treating this as fatal.
	return "", false
}
