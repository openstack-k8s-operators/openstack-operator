package util //nolint:revive // util is an acceptable package name in this context

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	ocpconfigv1 "github.com/openshift/api/config/v1"
	mc "github.com/openshift/api/machineconfiguration/v1"
	ocpicsp "github.com/openshift/api/operator/v1alpha1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// machineConfigIgnition - holds the relevant parts of the ignition file that we need to create
// the registries.conf on the dataplane nodes.
type machineConfigIgnition struct {
	Ignition struct {
		Version string `json:"version"`
	} `json:"ignition"`
	Storage struct {
		Files []struct {
			Contents struct {
				Compression string `json:"compression,omitempty"`
				Source      string `json:"source"`
			} `json:"contents"`
			Mode      int    `json:"mode,omitempty"`
			Overwrite bool   `json:"overwrite,omitempty"`
			Path      string `json:"path"`
		} `json:"files"`
	} `json:"storage"`
}

// HasMirrorRegistries checks if OCP has IDMS/ICSP mirror registries configured.
// Note: The presence of IDMS/ICSP doesn't necessarily mean the cluster is disconnected.
// Mirror registries may be configured for other reasons (performance, policy, caching, etc.).
// Returns false without error if the CRDs don't exist (non-OpenShift cluster).
func HasMirrorRegistries(ctx context.Context, helper *helper.Helper) (bool, error) {
	// Check IDMS first (current API), then fall back to ICSP (deprecated)
	idmsList := &ocpconfigv1.ImageDigestMirrorSetList{}
	if err := helper.GetClient().List(ctx, idmsList); err != nil {
		if !IsNoMatchError(err) {
			return false, err
		}
		// CRD doesn't exist, continue to check ICSP
	} else if len(idmsList.Items) > 0 {
		return true, nil
	}

	icspList := &ocpicsp.ImageContentSourcePolicyList{}
	if err := helper.GetClient().List(ctx, icspList); err != nil {
		if !IsNoMatchError(err) {
			return false, err
		}
		// CRD doesn't exist, fall through to return false
	} else if len(icspList.Items) > 0 {
		return true, nil
	}

	return false, nil
}

func sortedSetKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	result := make([]string, 0, len(set))
	for k := range set {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

// GetMirrorRegistryScopes returns the configured mirror scopes and their
// source registry mapping, preferring IDMS and falling back to ICSP.
// The returned scopes are normalized and de-duplicated for policy matching.
// The sourceByMirror map links each mirror scope back to its IDMS/ICSP source.
func GetMirrorRegistryScopes(ctx context.Context, helper *helper.Helper) ([]string, map[string]string, error) {
	idmsList := &ocpconfigv1.ImageDigestMirrorSetList{}
	if err := helper.GetClient().List(ctx, idmsList); err != nil {
		if !IsNoMatchError(err) {
			return nil, nil, err
		}
	} else {
		scopes := map[string]struct{}{}
		sourceByMirror := map[string]string{}
		for _, idms := range idmsList.Items {
			for _, mirrorSet := range idms.Spec.ImageDigestMirrors {
				source := normalizeImageScope(string(mirrorSet.Source))
				for _, mirror := range mirrorSet.Mirrors {
					m := normalizeImageScope(string(mirror))
					if m != "" {
						scopes[m] = struct{}{}
						if source != "" {
							sourceByMirror[m] = source
						}
					}
				}
			}
		}
		if result := sortedSetKeys(scopes); len(result) > 0 {
			return result, sourceByMirror, nil
		}
	}

	icspList := &ocpicsp.ImageContentSourcePolicyList{}
	if err := helper.GetClient().List(ctx, icspList); err != nil {
		if !IsNoMatchError(err) {
			return nil, nil, err
		}
	} else {
		scopes := map[string]struct{}{}
		sourceByMirror := map[string]string{}
		for _, icsp := range icspList.Items {
			for _, mirrorSet := range icsp.Spec.RepositoryDigestMirrors {
				source := normalizeImageScope(mirrorSet.Source)
				for _, mirror := range mirrorSet.Mirrors {
					m := normalizeImageScope(mirror)
					if m != "" {
						scopes[m] = struct{}{}
						if source != "" {
							sourceByMirror[m] = source
						}
					}
				}
			}
		}
		return sortedSetKeys(scopes), sourceByMirror, nil
	}

	return nil, nil, nil
}

// IsNoMatchError checks if the error indicates that a CRD/resource type doesn't exist
func IsNoMatchError(err error) bool {
	errStr := err.Error()
	// Check for "no matches for kind" type errors which indicate the CRD doesn't exist.
	// Also check for "no kind is registered" which occurs when the type isn't in the scheme.
	return strings.Contains(errStr, "no matches for kind") ||
		strings.Contains(errStr, "no kind is registered")
}

// GetMCRegistryConf - will unmarshal the MachineConfig ignition file the machineConfigIgnition object.
// This is then parsed and the base64 decoded string is returned.
func GetMCRegistryConf(ctx context.Context, helper *helper.Helper) (string, error) {
	var registriesConf string

	masterMachineConfig, err := getMachineConfig(ctx, helper)
	if err != nil {
		return registriesConf, err
	}

	config := machineConfigIgnition{}
	registriesConf, err = config.formatRegistriesConfString(&masterMachineConfig)
	if err != nil {
		return registriesConf, err
	}

	return registriesConf, nil
}

func (mci *machineConfigIgnition) removePrefixFromB64String() (string, error) {
	const b64Prefix string = "data:text/plain;charset=utf-8;base64,"
	if strings.HasPrefix(mci.Storage.Files[0].Contents.Source, b64Prefix) {
		return mci.Storage.Files[0].Contents.Source[len(b64Prefix):], nil
	}

	return "", fmt.Errorf("no b64prefix found in MachineConfig")
}

func (mci *machineConfigIgnition) formatRegistriesConfString(machineConfig *mc.MachineConfig) (string, error) {
	var (
		err             error
		rawConfigString string
		configString    []byte
	)

	err = json.Unmarshal([]byte(machineConfig.Spec.Config.Raw), &mci)
	if err != nil {
		return "", err
	}

	rawConfigString, err = mci.removePrefixFromB64String()
	if err != nil {
		return "", err
	}
	configString, err = base64.StdEncoding.DecodeString(rawConfigString)
	if err != nil {
		return "", err
	}

	return string(configString), nil
}

func masterMachineConfigBuilder(machineConfigRegistries string) mc.MachineConfig {
	return mc.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machineConfigRegistries,
			Namespace: "",
		},
	}
}

func getMachineConfig(ctx context.Context, helper *helper.Helper) (mc.MachineConfig, error) {
	const machineConfigRegistries string = "99-master-generated-registries"

	masterMachineConfig := masterMachineConfigBuilder(machineConfigRegistries)

	err := helper.GetClient().Get(ctx,
		types.NamespacedName{
			Name: masterMachineConfig.Name, Namespace: masterMachineConfig.Namespace,
		}, &masterMachineConfig)
	if err != nil {
		return masterMachineConfig, err
	}

	return masterMachineConfig, nil
}

// RegistryMapping pairs a mirror with its upstream source.
type RegistryMapping struct {
	Mirror       string `json:"mirror"`
	Source       string `json:"source"`
	SignedPrefix string `json:"signedPrefix,omitempty"`
}

// SigstorePolicyInfo contains the EDPM sigstore settings.
type SigstorePolicyInfo struct {
	RegistryMappings []RegistryMapping
	CosignKeyData    string
}

const (
	clusterImagePolicyCRDName      = "clusterimagepolicies.config.openshift.io"
	clusterImagePolicyGroup        = "config.openshift.io"
	clusterImagePolicyKind         = "ClusterImagePolicy"
	clusterImagePolicyV1           = "v1"
	clusterImagePolicyV1Alpha1     = "v1alpha1"
	publicKeyRootOfTrustPolicyType = "PublicKey"
	remapIdentityMatchPolicy       = "RemapIdentity"
)

func normalizeImageScope(scope string) string {
	return strings.TrimSuffix(strings.TrimSpace(scope), "/")
}

func clusterImagePolicyScopeMatchesMirror(policyScope string, mirrorScope string) bool {
	policyScope = normalizeImageScope(policyScope)
	mirrorScope = normalizeImageScope(mirrorScope)

	if policyScope == "" || mirrorScope == "" {
		return false
	}

	if strings.HasPrefix(policyScope, "*.") {
		mirrorHostPort := strings.SplitN(mirrorScope, "/", 2)[0]
		mirrorHost := strings.SplitN(mirrorHostPort, ":", 2)[0]
		suffix := strings.TrimPrefix(policyScope, "*")
		return strings.HasSuffix(mirrorHost, suffix)
	}

	return mirrorScope == policyScope || strings.HasPrefix(mirrorScope, policyScope+"/")
}

func getServedClusterImagePolicyVersion(ctx context.Context, helper *helper.Helper) (string, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: clusterImagePolicyCRDName}, crd); err != nil {
		if k8s_errors.IsNotFound(err) || IsNoMatchError(err) {
			return "", nil
		}
		return "", err
	}

	for _, preferredVersion := range []string{clusterImagePolicyV1, clusterImagePolicyV1Alpha1} {
		for _, version := range crd.Spec.Versions {
			if version.Name == preferredVersion && version.Served {
				return preferredVersion, nil
			}
		}
	}

	return "", nil
}

func listClusterImagePolicies(
	ctx context.Context,
	helper *helper.Helper,
	version string,
) (*unstructured.UnstructuredList, error) {
	// Use an unstructured client here because ClusterImagePolicy may be served as
	// either v1 or v1alpha1 depending on the cluster; binding to a typed v1 API
	// would fail on clusters that do not serve v1 yet.
	policyList := &unstructured.UnstructuredList{}
	policyList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   clusterImagePolicyGroup,
		Version: version,
		Kind:    clusterImagePolicyKind + "List",
	})

	if err := helper.GetClient().List(ctx, policyList); err != nil {
		if IsNoMatchError(err) {
			return nil, nil
		}
		return nil, err
	}

	return policyList, nil
}

func collectMatchedMirrorScopes(policyScopes []string, mirrorScopes []string) []string {
	matchedMirrorScopes := map[string]struct{}{}
	for _, scope := range policyScopes {
		policyScope := normalizeImageScope(scope)
		if policyScope == "" {
			continue
		}

		for _, mirrorScope := range mirrorScopes {
			if clusterImagePolicyScopeMatchesMirror(policyScope, mirrorScope) {
				matchedMirrorScopes[mirrorScope] = struct{}{}
			}
		}
	}

	return sortedSetKeys(matchedMirrorScopes)
}

// GetSigstoreImagePolicy checks if OCP has a ClusterImagePolicy configured
// with sigstore signature verification for one of the mirror registries in use.
// sourceByMirror maps each mirror scope to its upstream source registry (from IDMS/ICSP).
// Returns policy info if a relevant policy is found, nil if no policy exists.
// Returns nil without error if the ClusterImagePolicy CRD is not installed.
func GetSigstoreImagePolicy(ctx context.Context, helper *helper.Helper, mirrorScopes []string, sourceByMirror map[string]string) (*SigstorePolicyInfo, error) {
	if len(mirrorScopes) == 0 {
		return nil, nil
	}

	version, err := getServedClusterImagePolicyVersion(ctx, helper)
	if err != nil {
		return nil, err
	}
	if version == "" {
		return nil, nil
	}

	policyList, err := listClusterImagePolicies(ctx, helper, version)
	if err != nil {
		return nil, err
	}
	if policyList == nil {
		return nil, nil
	}

	type policyMatch struct {
		name         string
		keyData      string
		signedPrefix string
	}

	normalizedMirrors := make([]string, 0, len(mirrorScopes))
	sourceByNormalizedMirror := make(map[string]string, len(sourceByMirror))
	for _, mirrorScope := range mirrorScopes {
		normalizedMirror := normalizeImageScope(mirrorScope)
		if normalizedMirror == "" {
			continue
		}
		normalizedMirrors = append(normalizedMirrors, normalizedMirror)
		sourceByNormalizedMirror[normalizedMirror] = sourceByMirror[mirrorScope]
	}
	sort.Strings(normalizedMirrors)

	matchByMirror := map[string]policyMatch{}

	for _, policy := range policyList.Items {
		if policy.GetName() == "openshift" {
			continue
		}

		policyType, found, err := unstructured.NestedString(policy.Object, "spec", "policy", "rootOfTrust", "policyType")
		if err != nil {
			return nil, fmt.Errorf("failed to parse ClusterImagePolicy %s policyType: %w", policy.GetName(), err)
		}
		if !found || policyType != publicKeyRootOfTrustPolicyType {
			continue
		}

		keyData, found, err := unstructured.NestedString(policy.Object, "spec", "policy", "rootOfTrust", "publicKey", "keyData")
		if err != nil {
			return nil, fmt.Errorf("failed to parse ClusterImagePolicy %s keyData: %w", policy.GetName(), err)
		}
		if !found || len(keyData) == 0 {
			continue
		}

		scopes, found, err := unstructured.NestedStringSlice(policy.Object, "spec", "scopes")
		if err != nil {
			return nil, fmt.Errorf("failed to parse ClusterImagePolicy %s scopes: %w", policy.GetName(), err)
		}
		if !found || len(scopes) == 0 {
			continue
		}

		signedPrefix := ""
		matchPolicy, found, err := unstructured.NestedString(policy.Object, "spec", "policy", "signedIdentity", "matchPolicy")
		if err != nil {
			return nil, fmt.Errorf("failed to parse ClusterImagePolicy %s matchPolicy: %w", policy.GetName(), err)
		}
		if found && matchPolicy == remapIdentityMatchPolicy {
			signedPrefix, _, err = unstructured.NestedString(
				policy.Object,
				"spec", "policy", "signedIdentity", "remapIdentity", "signedPrefix",
			)
			if err != nil {
				return nil, fmt.Errorf("failed to parse ClusterImagePolicy %s signedPrefix: %w", policy.GetName(), err)
			}
		}

		matchedMirrors := collectMatchedMirrorScopes(scopes, normalizedMirrors)
		if len(matchedMirrors) == 0 {
			continue
		}

		match := policyMatch{
			name:         policy.GetName(),
			keyData:      keyData,
			signedPrefix: signedPrefix,
		}
		for _, mirror := range matchedMirrors {
			if existing, found := matchByMirror[mirror]; found {
				return nil, fmt.Errorf(
					"mirror scope %s matched multiple ClusterImagePolicies: %s, %s",
					mirror, existing.name, policy.GetName(),
				)
			}
			matchByMirror[mirror] = match
		}
	}

	if len(matchByMirror) == 0 {
		return nil, nil
	}

	sortedMirrors := make([]string, 0, len(matchByMirror))
	for mirror := range matchByMirror {
		sortedMirrors = append(sortedMirrors, mirror)
	}
	sort.Strings(sortedMirrors)

	firstMatch := matchByMirror[sortedMirrors[0]]
	match := &SigstorePolicyInfo{
		RegistryMappings: make([]RegistryMapping, 0, len(sortedMirrors)),
		CosignKeyData:    firstMatch.keyData,
	}

	for _, mirror := range sortedMirrors {
		mirrorMatch := matchByMirror[mirror]
		if match.CosignKeyData != mirrorMatch.keyData {
			return nil, fmt.Errorf(
				"matched ClusterImagePolicies use different cosign key data and cannot be combined: %s, %s",
				firstMatch.name, mirrorMatch.name,
			)
		}

		match.RegistryMappings = append(match.RegistryMappings, RegistryMapping{
			Mirror:       mirror,
			Source:       sourceByNormalizedMirror[mirror],
			SignedPrefix: mirrorMatch.signedPrefix,
		})
	}

	return match, nil
}

// GetMirrorRegistryCACerts retrieves CA certificates from image.config.openshift.io/cluster.
// Returns nil without error if:
//   - not on OpenShift (Image CRD doesn't exist)
//   - no additional CA is configured
//   - the referenced ConfigMap doesn't exist
func GetMirrorRegistryCACerts(ctx context.Context, helper *helper.Helper) (map[string]string, error) {
	imageConfig := &ocpconfigv1.Image{}
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "cluster"}, imageConfig); err != nil {
		if IsNoMatchError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get image.config.openshift.io/cluster: %w", err)
	}

	if imageConfig.Spec.AdditionalTrustedCA.Name == "" {
		return nil, nil
	}

	caConfigMap := &corev1.ConfigMap{}
	if err := helper.GetClient().Get(ctx, types.NamespacedName{
		Name:      imageConfig.Spec.AdditionalTrustedCA.Name,
		Namespace: "openshift-config",
	}, caConfigMap); err != nil {
		if k8s_errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get ConfigMap %s in openshift-config: %w",
			imageConfig.Spec.AdditionalTrustedCA.Name, err)
	}

	return caConfigMap.Data, nil
}
