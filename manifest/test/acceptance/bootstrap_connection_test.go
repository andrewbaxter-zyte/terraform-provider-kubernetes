// Copyright IBM Corp. 2017, 2026
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance
// +build acceptance

package acceptance

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/terraform-provider-kubernetes/manifest/provider"
	"github.com/hashicorp/terraform-provider-kubernetes/manifest/test/helper/kubernetes"
	tfstatehelper "github.com/hashicorp/terraform-provider-kubernetes/manifest/test/helper/state"
)

// TestKubernetesManifest_BootstrapConnection covers the "bootstrap_connection" block
// end to end, through real Terraform rather than a hand-built request.
//
// Two things here can only be observed from outside the provider, and both have broken
// in ways unit tests could not see:
//
//   - that Terraform delivers the block to the provider at all. "connection" is a
//     reserved meta-argument that Core consumes silently, so a block by that name
//     arrives empty with no error raised anywhere.
//   - that an unknown connection defers rather than fails. The provider cannot reach a
//     cluster to plan against, so it must report "object" as known-after-apply and do
//     the cluster-dependent work during the apply re-plan.
//
// The plan assertion below is what distinguishes the two failure modes: if the block
// were dropped, the resource would fall back to the provider's own configuration, the
// cluster would be reachable at plan time, and "object" would be fully known.
//
// Not covered here: the write-only credential attributes (token, password, client_key,
// client_certificate, exec.env). Those need Terraform 1.11+, and the acceptance matrix
// runs older versions too, so this fixture authenticates with config_path instead.
func TestKubernetesManifest_BootstrapConnection(t *testing.T) {
	ctx := context.Background()

	kubeconfig := os.Getenv("KUBE_CONFIG_PATH")
	if kubeconfig == "" {
		t.Skip("KUBE_CONFIG_PATH must be set for the bootstrap_connection acceptance test")
	}

	reattachInfo, err := provider.ServeTest(ctx, hclog.Default(), t)
	if err != nil {
		t.Errorf("Failed to create provider instance: %q", err)
	}

	name := randName()
	namespace := randName()

	tf := tfhelper.RequireNewWorkingDir(ctx, t)
	tf.SetReattachInfo(ctx, reattachInfo)
	defer func() {
		tf.Destroy(ctx)
		tf.Close()
		k8shelper.AssertNamespacedResourceDoesNotExist(t, "v1", "configmaps", namespace, name)
	}()

	k8shelper.CreateNamespace(t, namespace)
	defer k8shelper.DeleteResource(t, namespace, kubernetes.NewGroupVersionResource("v1", "namespaces"))

	tfvars := TFVARS{
		"namespace":  namespace,
		"name":       name,
		"kubeconfig": kubeconfig,
	}
	tfconfig := loadTerraformConfig(t, "BootstrapConnection/bootstrap_connection.tf", tfvars)
	tf.SetConfig(ctx, tfconfig)
	tf.Init(ctx)

	// The connection is not knowable at plan time, so planning must succeed with
	// "object" unknown rather than failing to reach a cluster.
	if err := tf.CreatePlan(ctx); err != nil {
		t.Fatalf("Failed to create plan: %q", err)
	}
	plan, err := tf.SavedPlan(ctx)
	if err != nil {
		t.Fatalf("Failed to retrieve saved plan: %q", err)
	}

	plannedIdx := -1
	for i, rc := range plan.ResourceChanges {
		if rc.Address == "kubernetes_manifest.test" {
			plannedIdx = i
			break
		}
	}
	if plannedIdx < 0 {
		t.Fatal("kubernetes_manifest.test not found in plan")
	}
	planned := plan.ResourceChanges[plannedIdx]
	if len(planned.Change.Actions) != 1 || planned.Change.Actions[0] != "create" {
		t.Fatalf("expected a create action, planned for: %q", planned.Change.Actions)
	}
	afterUnknown, ok := planned.Change.AfterUnknown.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected AfterUnknown shape: %T", planned.Change.AfterUnknown)
	}
	if unknown, found := afterUnknown["object"]; !found || unknown == false {
		t.Error(`expected "object" to be unknown at plan time while the connection is unknown`)
	}

	// The apply re-plan resolves the connection and does the cluster-dependent work.
	tf.Apply(ctx)

	k8shelper.AssertNamespacedResourceExists(t, "v1", "configmaps", namespace, name)

	s, err := tf.State(ctx)
	if err != nil {
		t.Fatalf("Failed to retrieve terraform state: %q", err)
	}
	tfstate := tfstatehelper.NewHelper(s)
	tfstate.AssertAttributeValues(t, tfstatehelper.AttributeValues{
		"kubernetes_manifest.test.object.metadata.namespace": namespace,
		"kubernetes_manifest.test.object.metadata.name":      name,
		"kubernetes_manifest.test.object.data.foo":           "bar",
	})
}
