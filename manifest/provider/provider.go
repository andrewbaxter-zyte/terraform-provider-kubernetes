// Copyright IBM Corp. 2017, 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// GetObjectTypeFromSchema returns a tftypes.Type that can wholy represent the schema input
func GetObjectTypeFromSchema(schema *tfprotov5.Schema) tftypes.Type {
	bm := map[string]tftypes.Type{}

	for _, att := range schema.Block.Attributes {
		bm[att.Name] = att.Type
	}

	for _, b := range schema.Block.BlockTypes {
		a := map[string]tftypes.Type{}
		for _, att := range b.Block.Attributes {
			a[att.Name] = att.Type
		}
		bm[b.TypeName] = tftypes.List{
			ElementType: tftypes.Object{AttributeTypes: a},
		}

		// FIXME we can make this function recursive to handle
		// n levels of nested blocks
		for _, bb := range b.Block.BlockTypes {
			aa := map[string]tftypes.Type{}
			for _, att := range bb.Block.Attributes {
				aa[att.Name] = att.Type
			}
			a[bb.TypeName] = tftypes.List{
				ElementType: tftypes.Object{AttributeTypes: aa},
			}
		}
	}

	return tftypes.Object{AttributeTypes: bm}
}

// GetResourceType returns the tftypes.Type of a resource of type 'name'
func GetResourceType(name string) (tftypes.Type, error) {
	sch := GetProviderResourceSchema()
	rsch, ok := sch[name]
	if !ok {
		return tftypes.DynamicPseudoType, fmt.Errorf("unknown resource %s - cannot find schema", name)
	}
	return GetObjectTypeFromSchema(rsch), nil
}

// GetDataSourceType returns the tftypes.Type of a datasource of type 'name'
func GetDataSourceType(name string) (tftypes.Type, error) {
	sch := GetProviderDataSourceSchema()
	rsch, ok := sch[name]
	if !ok {
		return tftypes.DynamicPseudoType, fmt.Errorf("unknown data source %q: cannot find schema", name)
	}
	return GetObjectTypeFromSchema(rsch), nil
}

func connectionBlockSchema() *tfprotov5.SchemaNestedBlock {
	return &tfprotov5.SchemaNestedBlock{
		TypeName: "bootstrap_connection",
		Nesting:  tfprotov5.SchemaNestedBlockNestingModeList,
		MinItems: 0,
		MaxItems: 1,
		Block: &tfprotov5.SchemaBlock{
			Description: "Connection settings used while this resource is being created, for a cluster that does not exist yet when the provider is configured and so cannot be reached by any provider configuration during that apply. Reading, replacing and destroying the resource use its provider instead, so the resource still needs a provider configured for the same cluster - to manage resources across several clusters, use aliased provider configurations rather than this block. Not merged with the provider block, and unlike the provider block it does not fall back to KUBE_* environment variables. The credential attributes are write-only: they are never persisted to state, so Terraform 1.11 or later is required to set them.",
			BlockTypes: []*tfprotov5.SchemaNestedBlock{
				{
					TypeName: "exec",
					Nesting:  tfprotov5.SchemaNestedBlockNestingModeList,
					MinItems: 0,
					MaxItems: 1,
					Block: &tfprotov5.SchemaBlock{
						Description: "Configuration block to use an exec-based credential plugin, e.g. call an external command to receive user credentials.",
						Attributes: []*tfprotov5.SchemaAttribute{
							{
								Name:        "api_version",
								Type:        tftypes.String,
								Required:    true,
								Description: "API version to use when decoding the ExecCredentials resource, e.g. `client.authentication.k8s.io/v1`.",
							},
							{
								Name:        "command",
								Type:        tftypes.String,
								Required:    true,
								Description: "Command to execute.",
							},
							{
								Name:        "env",
								Type:        tftypes.Map{ElementType: tftypes.String},
								Optional:    true,
								WriteOnly:   true,
								Sensitive:   true,
								Description: "Map of environment variables to set when executing the plugin.",
							},
							{
								Name:        "args",
								Type:        tftypes.List{ElementType: tftypes.String},
								Optional:    true,
								Description: "List of arguments to pass when executing the plugin.",
							},
						},
					},
				},
			},
			Attributes: []*tfprotov5.SchemaAttribute{
				{
					Name:        "host",
					Type:        tftypes.String,
					Optional:    true,
					Description: "The hostname (in form of URI) of the Kubernetes API.",
				},
				{
					Name:        "username",
					Type:        tftypes.String,
					Optional:    true,
					Description: "The username to use for HTTP basic authentication when accessing the Kubernetes API.",
				},
				{
					Name:        "password",
					Type:        tftypes.String,
					Optional:    true,
					WriteOnly:   true,
					Sensitive:   true,
					Description: "The password to use for HTTP basic authentication when accessing the Kubernetes API.",
				},
				{
					Name:        "insecure",
					Type:        tftypes.Bool,
					Optional:    true,
					Description: "Whether the server should be accessed without verifying the TLS certificate. Defaults to `false`.",
				},
				{
					Name:        "tls_server_name",
					Type:        tftypes.String,
					Optional:    true,
					Description: "Server name passed to the server for SNI and is used in the client to check server certificates against.",
				},
				{
					Name:        "client_certificate",
					Type:        tftypes.String,
					Optional:    true,
					WriteOnly:   true,
					Sensitive:   true,
					Description: "PEM-encoded client certificate for TLS authentication.",
				},
				{
					Name:        "client_key",
					Type:        tftypes.String,
					Optional:    true,
					WriteOnly:   true,
					Sensitive:   true,
					Description: "PEM-encoded client certificate key for TLS authentication.",
				},
				{
					Name:        "cluster_ca_certificate",
					Type:        tftypes.String,
					Optional:    true,
					Description: "PEM-encoded root certificates bundle for TLS authentication.",
				},
				{
					Name:        "config_paths",
					Type:        tftypes.List{ElementType: tftypes.String},
					Optional:    true,
					Description: "A list of paths to the kube config files.",
				},
				{
					Name:        "config_path",
					Type:        tftypes.String,
					Optional:    true,
					Description: "A path to a kube config file.",
				},
				{
					Name:        "config_context",
					Type:        tftypes.String,
					Optional:    true,
					Description: "Context to choose from the config file.",
				},
				{
					Name:        "config_context_cluster",
					Type:        tftypes.String,
					Optional:    true,
					Description: "Cluster context of the kube config (name of the kubeconfig cluster, `--cluster` flag in `kubectl`).",
				},
				{
					Name:        "config_context_auth_info",
					Type:        tftypes.String,
					Optional:    true,
					Description: "Authentication info context of the kube config (name of the kubeconfig user, `--user` flag in `kubectl`).",
				},
				{
					Name:        "token",
					Type:        tftypes.String,
					Optional:    true,
					WriteOnly:   true,
					Sensitive:   true,
					Description: "Token of your service account.",
				},
				{
					Name:        "proxy_url",
					Type:        tftypes.String,
					Optional:    true,
					Description: "URL to the proxy to be used for all API requests. URLs with \"http\", \"https\", and \"socks5\" schemes are supported.",
				},
			},
		},
	}
}

// writeOnlyConnectionAttributes lists the attributes of the "bootstrap_connection" block that are
// declared WriteOnly, by path within the block. Terraform Core requires write-only values
// to be null in every state and plan a provider returns, so these have to be stripped
// from every response that carries resource state.
var writeOnlyConnectionAttributes = map[string][]string{
	"":     {"token", "password", "client_key", "client_certificate"},
	"exec": {"env"},
}

// writeOnlyConnectionAttributesSet returns the names of the write-only "bootstrap_connection"
// attributes that the given resource configuration actually sets, qualified by nested
// block where applicable (e.g. "exec.env"). Used to report what needs to change.
func writeOnlyConnectionAttributesSet(resourceConfig map[string]tftypes.Value) []string {
	connVal, ok := resourceConfig["bootstrap_connection"]
	if !ok || connVal.IsNull() {
		return nil
	}
	var connBlock []tftypes.Value
	if err := connVal.As(&connBlock); err != nil || len(connBlock) == 0 || connBlock[0].IsNull() {
		return nil
	}
	var connMap map[string]tftypes.Value
	if err := connBlock[0].As(&connMap); err != nil {
		return nil
	}

	var set []string
	for _, name := range writeOnlyConnectionAttributes[""] {
		if v, ok := connMap[name]; ok && !v.IsNull() {
			set = append(set, name)
		}
	}

	execVal, ok := connMap["exec"]
	if !ok || execVal.IsNull() {
		return set
	}
	var execBlock []tftypes.Value
	if err := execVal.As(&execBlock); err != nil || len(execBlock) == 0 || execBlock[0].IsNull() {
		return set
	}
	var execMap map[string]tftypes.Value
	if err := execBlock[0].As(&execMap); err != nil {
		return set
	}
	for _, name := range writeOnlyConnectionAttributes["exec"] {
		if v, ok := execMap[name]; ok && !v.IsNull() {
			set = append(set, "exec."+name)
		}
	}
	return set
}

// nullifyWriteOnlyConnection returns v with every write-only attribute of the
// "bootstrap_connection" block set to null.
//
// Terraform Core treats a non-null write-only value in a response as a data consistency
// error, so this must be applied to any planned state or new state before it is returned.
// It is also what keeps credentials out of state entirely: a "bootstrap_connection" block supplies
// credentials for the run that creates a resource and nothing more, so there is nothing
// recorded for a later run to authenticate with - or for an operator to read out of a
// state file.
func nullifyWriteOnlyConnection(v tftypes.Value) (tftypes.Value, error) {
	if v.IsNull() || !v.IsKnown() {
		return v, nil
	}
	return tftypes.Transform(v, func(path *tftypes.AttributePath, val tftypes.Value) (tftypes.Value, error) {
		steps := path.Steps()
		// connection -> [index] -> attr  (3 steps), or
		// connection -> [index] -> exec -> [index] -> attr  (5 steps)
		if len(steps) != 3 && len(steps) != 5 {
			return val, nil
		}
		root, ok := steps[0].(tftypes.AttributeName)
		if !ok || string(root) != "bootstrap_connection" {
			return val, nil
		}

		block := ""
		if len(steps) == 5 {
			nested, ok := steps[2].(tftypes.AttributeName)
			if !ok {
				return val, nil
			}
			block = string(nested)
		}
		attr, ok := steps[len(steps)-1].(tftypes.AttributeName)
		if !ok {
			return val, nil
		}

		for _, name := range writeOnlyConnectionAttributes[block] {
			if string(attr) == name {
				if val.IsNull() {
					return val, nil
				}
				return tftypes.NewValue(val.Type(), nil), nil
			}
		}
		return val, nil
	})
}

// GetProviderResourceSchema contains the definitions of all supported resources
func GetProviderResourceSchema() map[string]*tfprotov5.Schema {
	return map[string]*tfprotov5.Schema{
		"kubernetes_manifest": {
			Version: 1,
			Block: &tfprotov5.SchemaBlock{
				BlockTypes: []*tfprotov5.SchemaNestedBlock{
					{
						TypeName: "timeouts",
						Nesting:  tfprotov5.SchemaNestedBlockNestingModeList,
						MinItems: 0,
						MaxItems: 1,
						Block: &tfprotov5.SchemaBlock{
							Attributes: []*tfprotov5.SchemaAttribute{
								{
									Name:        "create",
									Type:        tftypes.String,
									Description: "Timeout for the create operation.",
									Optional:    true,
								},
								{
									Name:        "update",
									Type:        tftypes.String,
									Description: "Timeout for the update operation.",
									Optional:    true,
								},
								{
									Name:        "delete",
									Type:        tftypes.String,
									Description: "Timeout for the delete operation.",
									Optional:    true,
								},
							},
						},
					},
					{
						TypeName: "field_manager",
						Nesting:  tfprotov5.SchemaNestedBlockNestingModeList,
						MinItems: 0,
						MaxItems: 1,
						Block: &tfprotov5.SchemaBlock{
							Description: "Configure field manager options.",
							Attributes: []*tfprotov5.SchemaAttribute{
								{
									Name:            "name",
									Type:            tftypes.String,
									Required:        false,
									Optional:        true,
									Computed:        false,
									Sensitive:       false,
									Description:     "The name to use for the field manager when creating and updating the resource.",
									DescriptionKind: 0,
									Deprecated:      false,
								},
								{
									Name:            "force_conflicts",
									Type:            tftypes.Bool,
									Required:        false,
									Optional:        true,
									Computed:        false,
									Sensitive:       false,
									Description:     "Force changes against conflicts.",
									DescriptionKind: 0,
									Deprecated:      false,
								},
							},
						},
					},
					{
						TypeName: "wait",
						Nesting:  tfprotov5.SchemaNestedBlockNestingModeList,
						MinItems: 0,
						MaxItems: 1,
						Block: &tfprotov5.SchemaBlock{
							Description: "Configure waiter options.",
							BlockTypes: []*tfprotov5.SchemaNestedBlock{
								{
									TypeName: "condition",
									Nesting:  tfprotov5.SchemaNestedBlockNestingModeList,
									MinItems: 0,
									Block: &tfprotov5.SchemaBlock{
										Attributes: []*tfprotov5.SchemaAttribute{
											{
												Name:        "status",
												Type:        tftypes.String,
												Optional:    true,
												Description: "The condition status.",
											}, {
												Name:        "type",
												Type:        tftypes.String,
												Optional:    true,
												Description: "The type of condition.",
											},
										},
									},
								},
							},
							Attributes: []*tfprotov5.SchemaAttribute{
								{
									Name:        "rollout",
									Type:        tftypes.Bool,
									Optional:    true,
									Description: "Wait for rollout to complete on resources that support `kubectl rollout status`.",
								},
								{
									Name:        "fields",
									Type:        tftypes.Map{ElementType: tftypes.String},
									Optional:    true,
									Description: "A map of paths to fields to wait for a specific field value.",
								},
							},
						},
					},
					connectionBlockSchema(),
				},
				Attributes: []*tfprotov5.SchemaAttribute{
					{
						Name:        "manifest",
						Type:        tftypes.DynamicPseudoType,
						Required:    true,
						Description: "A Kubernetes manifest describing the desired state of the resource in HCL format.",
					},
					{
						Name:        "object",
						Type:        tftypes.DynamicPseudoType,
						Optional:    true,
						Computed:    true,
						Description: "The resulting resource state, as returned by the API server after applying the desired state from `manifest`.",
					},
					{
						Name: "wait_for",
						Type: tftypes.Object{
							AttributeTypes: map[string]tftypes.Type{
								"fields": tftypes.Map{
									ElementType: tftypes.String,
								},
							},
						},
						Optional:    true,
						Deprecated:  true,
						Description: "A map of attribute paths and desired patterns to be matched. After each apply the provider will wait for all attributes listed here to reach a value that matches the desired pattern.",
					},
					{
						Name:        "computed_fields",
						Type:        tftypes.List{ElementType: tftypes.String},
						Description: "List of manifest fields whose values can be altered by the API server during 'apply'. Defaults to: [\"metadata.annotations\", \"metadata.labels\"]",
						Optional:    true,
					},
				},
			},
		},
	}
}

// GetProviderDataSourceSchema contains the definitions of all supported data sources
func GetProviderDataSourceSchema() map[string]*tfprotov5.Schema {
	return map[string]*tfprotov5.Schema{
		"kubernetes_resource": {
			Version: 1,
			Block: &tfprotov5.SchemaBlock{
				Attributes: []*tfprotov5.SchemaAttribute{
					{
						Name:        "api_version",
						Type:        tftypes.String,
						Required:    true,
						Description: "The resource apiVersion.",
					},
					{
						Name:        "kind",
						Type:        tftypes.String,
						Required:    true,
						Description: "The resource kind.",
					},
					{
						Name:        "object",
						Type:        tftypes.DynamicPseudoType,
						Optional:    true,
						Computed:    true,
						Description: "The response from the API server.",
					},
				},
				BlockTypes: []*tfprotov5.SchemaNestedBlock{
					{
						TypeName: "metadata",
						Nesting:  tfprotov5.SchemaNestedBlockNestingModeList,
						MinItems: 1,
						MaxItems: 1,
						Block: &tfprotov5.SchemaBlock{
							Description: "Metadata for the resource",
							Attributes: []*tfprotov5.SchemaAttribute{
								{
									Name:        "name",
									Type:        tftypes.String,
									Required:    true,
									Description: "The resource name.",
								},
								{
									Name:        "namespace",
									Type:        tftypes.String,
									Optional:    true,
									Description: "The resource namespace.",
								},
							},
						},
					},
					connectionBlockSchema(),
				},
			},
		},
		"kubernetes_resources": {
			Version: 1,
			Block: &tfprotov5.SchemaBlock{
				Attributes: []*tfprotov5.SchemaAttribute{
					{
						Name:        "api_version",
						Type:        tftypes.String,
						Required:    true,
						Description: "The resource apiVersion.",
					},
					{
						Name:        "kind",
						Type:        tftypes.String,
						Required:    true,
						Description: "The resource kind.",
					},
					{
						Name:        "objects",
						Type:        tftypes.DynamicPseudoType,
						Optional:    true,
						Computed:    true,
						Description: "The response from the API server.",
					},
					{
						Name:        "namespace",
						Type:        tftypes.String,
						Optional:    true,
						Description: "The resource namespace.",
					},
					{
						Name:        "field_selector",
						Type:        tftypes.String,
						Optional:    true,
						Description: "A selector to restrict the list of returned objects by their fields.",
					},
					{
						Name:        "label_selector",
						Type:        tftypes.String,
						Optional:    true,
						Description: "A selector to restrict the list of returned objects by their labels.",
					},
					{
						Name:        "limit",
						Type:        tftypes.Number,
						Optional:    true,
						Description: "Limit is a maximum number of responses to return for a list call.",
					},
				},
				BlockTypes: []*tfprotov5.SchemaNestedBlock{
					connectionBlockSchema(),
				},
			},
		},
	}
}
