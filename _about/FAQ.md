# Frequently Asked Questions

### Why isn’t my PR merged yet?

Unfortunately, due to the volume of issues and new pull requests we receive, we are unable to give each one the full attention that we would like. We do our best to focus on the contributions that provide the greatest value to the most community members.

### How do you decide what gets merged for each release?

The number one factor we look at when deciding what issues to look at are your 👍 [reactions](https://blog.github.com/2016-03-10-add-reactions-to-pull-requests-issues-and-comments/) to the original issue/PR description as these can be easily discovered. Comments that further explain desired use cases or poor user experience are also heavily factored. The items with the most support are always on our radar, and we do our best to keep the community updated on their status and potential timelines.

We also are investing time to improve the contributing experience by improving documentation.


### Backward Compatibility Promise

Our policy is described on the Terraform website [here](https://www.terraform.io/docs/extend/best-practices/versioning.html). While we do our best to prevent breaking changes until major version releases of the provider, it is generally recommended to [pin the provider version in your configuration](https://www.terraform.io/docs/configuration/providers.html#provider-versions).

Due to the constant release pace of Kubernetes and the relatively infrequent major version releases of the provider, there can be cases where a minor version update may contain unexpected changes depending on your configuration or environment.

### Why is not recommended to create Kubernetes resources in the same apply as the cluster?

When using resource attributes to pass credentials to the provider block from resources such as `aws_eks_cluster` and `google_container_cluster`, these resources should not be created in the same Terraform apply operation as Kubernetes provider resources. This will lead to intermittent and unpredictable errors which are hard to debug and diagnose. The root issue lies with the order in which Terraform itself evaluates the provider blocks vs. resources. Please refer to the [Provider Configuratopm](https://developer.hashicorp.com/terraform/language/providers/configuration#provider-configuration) section of the Terraform docs for more information.

For the `kubernetes_manifest` resource specifically, this resource _requires_ a Kubernetes cluster to already be available, as it needs to be able to fetch the OpenAPI spec from the Kubernetes API to generate the Terraform schema information needed to create a plan. 

For this reason, the most reliable way to configure the Kubernetes provider is to ensure that the cluster itself and the Kubernetes provider resources can each be managed with separate apply operations. We recommend using the corresponding data sources to supply values to the provider block as needed.

Two narrower workarounds also exist, for different variants of this problem:

- If your Terraform version supports [deferred actions](https://developer.hashicorp.com/terraform/language/v1.9.x/upgrade-guides#deferred-changes-in-terraform-plans), the provider will defer planning `kubernetes_manifest` resources (and the `kubernetes`/`kubernetes-alpha` provider blocks) until the referenced cluster's attributes are known, rather than failing. See `_examples/deferred-actions` for a worked example.
- If you need to apply a manifest to a cluster created earlier in the same apply, `kubernetes_manifest` (and the `kubernetes_resource`/`kubernetes_resources` data sources) accept an optional `bootstrap_connection` block supplying credentials for that cluster while the resource is being created, which no provider configuration can do. See the [`kubernetes_manifest` documentation](../docs/resources/manifest.md#bootstrapping-a-cluster-created-in-the-same-apply) for details. Unlike deferred actions, this doesn't require any particular Terraform version: when `bootstrap_connection` references a not-yet-known value, the provider reports `object` as "known after apply" instead of failing, and the re-plan Terraform performs during apply - once the credentials exist - does the cluster-dependent work. It only covers creation, though: reading, replacing and destroying the resource use its provider, so the resource still needs a provider configured for that cluster. The credential attributes are write-only and never reach state, which is what makes that split hold - and means a token from a short-lived data source leaves neither a permanent diff nor a secret at rest. Setting them requires Terraform 1.11 or later. Note this is *not* the way to manage resources across several clusters - declare one aliased `kubernetes` provider per cluster for that.

### How can I help?

Check out the [Contributing Guide](CONTRIBUTING.md) for additional information.

### How can I become a maintainer?

This is an area under active research. Stay tuned!
