/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	corev1alpha1 "github.com/user-cube/bootchain-operator/api/v1alpha1"
)

var bootdependencylog = logf.Log.WithName("bootdependency-webhook")

// +kubebuilder:webhook:path=/validate-core-bootchain-operator-ruicoelho-dev-v1alpha1-bootdependency,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.bootchain-operator.ruicoelho.dev,resources=bootdependencies,verbs=create;update,versions=v1alpha1,name=vbootdependency-v1alpha1.kb.io,admissionReviewVersions=v1

// BootDependencyCustomValidator validates BootDependency resources on create and update.
// It detects circular dependencies across the namespace using DFS.
type BootDependencyCustomValidator struct {
	Client client.Client
}

// SetupBootDependencyWebhookWithManager registers the webhook for BootDependency in the manager.
func SetupBootDependencyWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &corev1alpha1.BootDependency{}).
		WithValidator(&BootDependencyCustomValidator{Client: mgr.GetClient()}).
		Complete()
}

func (v *BootDependencyCustomValidator) ValidateCreate(ctx context.Context, bd *corev1alpha1.BootDependency) (admission.Warnings, error) {
	bootdependencylog.Info("Validating BootDependency create", "name", bd.Name, "namespace", bd.Namespace)
	return v.validate(ctx, bd)
}

func (v *BootDependencyCustomValidator) ValidateUpdate(ctx context.Context, _ *corev1alpha1.BootDependency, newBD *corev1alpha1.BootDependency) (admission.Warnings, error) {
	bootdependencylog.Info("Validating BootDependency update", "name", newBD.Name, "namespace", newBD.Namespace)
	return v.validate(ctx, newBD)
}

func (v *BootDependencyCustomValidator) ValidateDelete(_ context.Context, _ *corev1alpha1.BootDependency) (admission.Warnings, error) {
	return nil, nil
}

// validate checks for circular dependencies in the BootDependency graph for the namespace.
func (v *BootDependencyCustomValidator) validate(ctx context.Context, bd *corev1alpha1.BootDependency) (admission.Warnings, error) {
	// Build a map of all BootDependency objects in the namespace, including the one being created/updated.
	graph, err := v.buildGraph(ctx, bd)
	if err != nil {
		return nil, err
	}

	if cycle := detectCycle(bd.Name, graph); cycle != nil {
		path := strings.Join(cycle, " → ")
		return nil, field.Invalid(
			field.NewPath("spec", "dependsOn"),
			bd.Spec.DependsOn,
			fmt.Sprintf("circular dependency detected: %s", path),
		)
	}

	return nil, nil
}

// buildGraph returns a map of serviceName → list of service names it depends on,
// for all BootDependency objects in the same namespace. The incoming bd takes
// precedence over any existing object with the same name (handles updates).
func (v *BootDependencyCustomValidator) buildGraph(ctx context.Context, bd *corev1alpha1.BootDependency) (map[string][]string, error) {
	var list corev1alpha1.BootDependencyList
	if err := v.Client.List(ctx, &list, client.InNamespace(bd.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list BootDependencies: %w", err)
	}

	graph := make(map[string][]string)

	for i := range list.Items {
		existing := &list.Items[i]
		if existing.Name == bd.Name {
			// Skip — the incoming bd will overwrite this entry below.
			continue
		}
		// Only in-cluster service deps participate in the cycle graph.
		// External host deps are leaf nodes and can never form a BootDependency cycle.
		deps := make([]string, 0, len(existing.Spec.DependsOn))
		for _, dep := range existing.Spec.DependsOn {
			if dep.Service != "" {
				deps = append(deps, dep.Service)
			}
		}
		graph[existing.Name] = deps
	}

	// Add the incoming object (create or update).
	deps := make([]string, 0, len(bd.Spec.DependsOn))
	for _, dep := range bd.Spec.DependsOn {
		if dep.Service != "" {
			deps = append(deps, dep.Service)
		}
	}
	graph[bd.Name] = deps

	return graph, nil
}

// detectCycle runs a DFS from `start` and returns the cycle path if one is found,
// or nil if the graph is acyclic from that node.
func detectCycle(start string, graph map[string][]string) []string {
	visited := make(map[string]bool)
	path := make([]string, 0)

	var dfs func(node string) []string
	dfs = func(node string) []string {
		if visited[node] {
			// Found the cycle — return the path from where we first saw this node.
			for i, n := range path {
				if n == node {
					cycle := make([]string, len(path)-i+1)
					copy(cycle, path[i:])
					cycle[len(cycle)-1] = node
					return cycle
				}
			}
			return []string{node}
		}

		visited[node] = true
		path = append(path, node)

		for _, neighbour := range graph[node] {
			if cycle := dfs(neighbour); cycle != nil {
				return cycle
			}
		}

		path = path[:len(path)-1]
		visited[node] = false
		return nil
	}

	return dfs(start)
}
