// Copyright IBM Corp. 2017, 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-provider-kubernetes/manifest/openapi"
	"github.com/mitchellh/hashstructure"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

type resourceClients struct {
	ps           *RawProviderServer
	clientConfig *rest.Config
	key          uint64
}

// connectionFromConfig extracts the "bootstrap_connection" block from a resource's raw
// Config.
//
// Terraform Core always nulls write-only attributes (token, password, client_key,
// client_certificate, exec.env) when constructing ProposedNewState, PriorState and
// PlannedState - their real values are only ever present in Config. Plan and Apply must
// read the "bootstrap_connection" block from here, or a connection relying on one of
// those attributes would silently build a client with no credentials at all.
func connectionFromConfig(cfg *tfprotov5.DynamicValue, rt tftypes.Type) (tftypes.Value, error) {
	if cfg == nil {
		return tftypes.Value{}, fmt.Errorf("no config available")
	}
	cfgState, err := cfg.Unmarshal(rt)
	if err != nil {
		return tftypes.Value{}, err
	}
	cfgVal := make(map[string]tftypes.Value)
	if err := cfgState.As(&cfgVal); err != nil {
		return tftypes.Value{}, err
	}
	v, ok := cfgVal["bootstrap_connection"]
	if !ok {
		return tftypes.Value{}, fmt.Errorf("config has no \"bootstrap_connection\" attribute")
	}
	return v, nil
}

// hasConnectionBlock reports whether a resource's config or state carries a non-empty
// "bootstrap_connection" block.
func hasConnectionBlock(resourceConfig map[string]tftypes.Value) bool {
	connVal, ok := resourceConfig["bootstrap_connection"]
	if !ok || connVal.IsNull() || !connVal.IsKnown() {
		return false
	}
	var connBlock []tftypes.Value
	if err := connVal.As(&connBlock); err != nil {
		return false
	}
	return len(connBlock) > 0 && !connBlock[0].IsNull()
}

// providerClients returns clients built from the provider block's configuration.
//
// The "bootstrap_connection" block is a bootstrap shim: it supplies credentials for the run that
// creates a resource, when the provider cannot yet be configured because its settings
// depend on a resource being created in that same run. Every operation after that -
// read, destroy, the delete half of a replacement, state upgrade - uses the resource's
// provider instead, which is re-evaluated on every run and so cannot go stale. Reading
// the connection back out of state would mean using whatever credentials were recorded
// at the last apply, which for the short-lived tokens these clusters issue are long
// expired by the time they are needed.
//
// stateOrConfig is used only to tell the caller apart from an ordinary resource, so the
// diagnostic can explain the requirement.
func (ps *RawProviderServer) providerClients(stateOrConfig map[string]tftypes.Value) (*resourceClients, []*tfprotov5.Diagnostic) {
	if ps.clientConfig == nil && hasConnectionBlock(stateOrConfig) {
		return nil, []*tfprotov5.Diagnostic{{
			Severity: tfprotov5.DiagnosticSeverityError,
			Summary:  "Provider configuration required for this operation",
			Detail: `This resource has a "bootstrap_connection" block, but the provider block it belongs to has no ` +
				`Kubernetes configuration.` + "\n\n" +
				`A "bootstrap_connection" block only supplies credentials while the resource is being created, for the ` +
				`case where the target cluster does not exist yet and so the provider cannot be configured ` +
				`against it. Reading, updating or destroying the resource afterwards uses the provider, so ` +
				`that it always authenticates with freshly evaluated credentials rather than whatever was ` +
				`recorded in state at the last apply.` + "\n\n" +
				`Configure a provider for the target cluster - typically an aliased "kubernetes" provider ` +
				`whose settings come from a data source - and associate this resource with it using the ` +
				`"provider" meta-argument.`,
		}}
	}
	return &resourceClients{ps: ps}, nil
}

func (ps *RawProviderServer) clientsFor(resourceConfig map[string]tftypes.Value) (clients *resourceClients, connectionUnknown bool, diags []*tfprotov5.Diagnostic) {
	connVal, ok := resourceConfig["bootstrap_connection"]
	if !ok || connVal.IsNull() {
		return &resourceClients{ps: ps}, false, nil
	}
	if !connVal.IsFullyKnown() {
		return nil, true, nil
	}

	var connBlock []tftypes.Value
	if err := connVal.As(&connBlock); err != nil {
		return nil, false, []*tfprotov5.Diagnostic{{
			Severity: tfprotov5.DiagnosticSeverityError,
			Summary:  `Failed to assert type of "bootstrap_connection" block`,
			Detail:   err.Error(),
		}}
	}
	if len(connBlock) == 0 || connBlock[0].IsNull() {
		return &resourceClients{ps: ps}, false, nil
	}

	var connMap map[string]tftypes.Value
	if err := connBlock[0].As(&connMap); err != nil {
		return nil, false, []*tfprotov5.Diagnostic{{
			Severity: tfprotov5.DiagnosticSeverityError,
			Summary:  `Failed to extract "bootstrap_connection" block attributes`,
			Detail:   err.Error(),
		}}
	}

	// noEnvLookup: unlike the provider block, a "bootstrap_connection" block does not fall back to
	// KUBE_* environment variables. It must fully describe the cluster it targets.
	clientConfig, fp, buildDiags := buildClientConfig(connMap, ps.logger, noEnvLookup)
	if len(buildDiags) > 0 {
		return nil, false, buildDiags
	}
	if clientConfig == nil {
		return nil, false, []*tfprotov5.Diagnostic{{
			Severity: tfprotov5.DiagnosticSeverityError,
			Summary:  `Invalid "bootstrap_connection" block`,
			Detail: `The "bootstrap_connection" block did not resolve to a usable Kubernetes client configuration. ` +
				`Note that, unlike the provider block, a "bootstrap_connection" block does not read KUBE_* environment ` +
				`variables: it must specify the cluster's address and credentials itself (for example "host" ` +
				`plus "cluster_ca_certificate" and "token", or "config_path").`,
		}}
	}

	key, err := hashstructure.Hash(fp, nil)
	if err != nil {
		return nil, false, []*tfprotov5.Diagnostic{{
			Severity: tfprotov5.DiagnosticSeverityError,
			Summary:  `Failed to hash "bootstrap_connection" block`,
			Detail:   err.Error(),
		}}
	}

	return &resourceClients{ps: ps, clientConfig: clientConfig, key: key}, false, nil
}

func (rc *resourceClients) DynamicClient() (dynamic.Interface, error) {
	if rc.clientConfig == nil {
		return rc.ps.getDynamicClient()
	}
	return rc.ps.dynamicClients.Get(rc.key, func() (dynamic.Interface, error) {
		return dynamic.NewForConfig(rc.clientConfig)
	})
}

func (rc *resourceClients) DiscoveryClient() (discovery.DiscoveryInterface, error) {
	if rc.clientConfig == nil {
		return rc.ps.getDiscoveryClient()
	}
	return rc.ps.discoveryClients.Get(rc.key, func() (discovery.DiscoveryInterface, error) {
		return discovery.NewDiscoveryClientForConfig(rc.clientConfig)
	})
}

func (rc *resourceClients) RestMapper() (meta.RESTMapper, error) {
	if rc.clientConfig == nil {
		return rc.ps.getRestMapper()
	}
	return rc.ps.restMappers.Get(rc.key, func() (meta.RESTMapper, error) {
		dc, err := rc.DiscoveryClient()
		if err != nil {
			return nil, err
		}
		cacheClient := memory.NewMemCacheClient(dc)
		return restmapper.NewDeferredDiscoveryRESTMapper(cacheClient), nil
	})
}

func (rc *resourceClients) RestClient() (rest.Interface, error) {
	if rc.clientConfig == nil {
		return rc.ps.getRestClient()
	}
	return rc.ps.restClients.Get(rc.key, func() (rest.Interface, error) {
		return rest.UnversionedRESTClientFor(rc.clientConfig)
	})
}

func (rc *resourceClients) OAPIFoundry() (openapi.Foundry, error) {
	if rc.clientConfig == nil {
		return rc.ps.getOAPIv2Foundry()
	}
	return rc.ps.oapiFoundries.Get(rc.key, func() (openapi.Foundry, error) {
		rcl, err := rc.RestClient()
		if err != nil {
			return nil, fmt.Errorf("failed get OpenAPI spec: %s", err)
		}

		rq := rcl.Verb("GET").Timeout(30*time.Second).AbsPath("openapi", "v2")
		rs, err := rq.DoRaw(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("failed get OpenAPI spec: %s", err)
		}

		oapif, err := openapi.NewFoundryFromSpecV2(rs)
		if err != nil {
			return nil, fmt.Errorf("failed construct OpenAPI foundry: %s", err)
		}

		return oapif, nil
	})
}

func (rc *resourceClients) CheckValidCredentials(ctx context.Context) []*tfprotov5.Diagnostic {
	if rc.clientConfig == nil {
		return rc.ps.checkValidCredentials(ctx)
	}
	diagnostics, _ := rc.ps.checkValidCredentialsResults.Get(rc.key, func() (diags []*tfprotov5.Diagnostic, err error) {
		rcl, err := rc.RestClient()
		if err != nil {
			diags = append(diags, &tfprotov5.Diagnostic{
				Severity: tfprotov5.DiagnosticSeverityError,
				Summary:  "Failed to construct REST client",
				Detail:   err.Error(),
			})
			return
		}
		vpath := []string{"/apis"}
		rs := rcl.Get().AbsPath(vpath...).Do(ctx)
		if rs.Error() != nil {
			switch {
			case apierrors.IsUnauthorized(rs.Error()):
				diags = append(diags, &tfprotov5.Diagnostic{
					Severity: tfprotov5.DiagnosticSeverityError,
					Summary:  "Invalid credentials",
					Detail:   fmt.Sprintf("The credentials configured in the resource's 'connection' block are not accepted by the API server. Error: %s\n\nSet TF_LOG=debug and look for '[InvalidClientConfiguration]' in the log to see actual configuration.", rs.Error().Error()),
				})
			default:
				diags = append(diags, &tfprotov5.Diagnostic{
					Severity: tfprotov5.DiagnosticSeverityError,
					Summary:  "Invalid configuration for API client",
					Detail:   rs.Error().Error(),
				})
			}
			rc.ps.logger.Debug("[InvalidClientConfiguration]", "Config", dump(rc.clientConfig))
		}
		return
	})
	return diagnostics
}
