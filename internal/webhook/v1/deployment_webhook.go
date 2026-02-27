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

package v1

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	corev1alpha1 "github.com/user-cube/bootchain-operator/api/v1alpha1"
)

var deploymentlog = logf.Log.WithName("deployment-webhook")

// +kubebuilder:webhook:path=/mutate-apps-v1-deployment,mutating=true,failurePolicy=fail,sideEffects=None,groups=apps,resources=deployments,verbs=create;update,versions=v1,name=mdeployment-v1.kb.io,admissionReviewVersions=v1

// DeploymentCustomDefaulter injects init containers into Deployments based on
// a BootDependency resource with the same name in the same namespace.
type DeploymentCustomDefaulter struct {
	Client client.Client
}

// SetupDeploymentWebhookWithManager registers the webhook for Deployment in the manager.
func SetupDeploymentWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &appsv1.Deployment{}).
		WithDefaulter(&DeploymentCustomDefaulter{Client: mgr.GetClient()}).
		Complete()
}

// Default implements webhook.CustomDefaulter.
func (d *DeploymentCustomDefaulter) Default(ctx context.Context, obj *appsv1.Deployment) error {
	log := deploymentlog.WithValues("deployment", obj.Name, "namespace", obj.Namespace)

	var bd corev1alpha1.BootDependency
	err := d.Client.Get(ctx, types.NamespacedName{
		Name:      obj.Name,
		Namespace: obj.Namespace,
	}, &bd)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			// No BootDependency for this Deployment — nothing to inject.
			return nil
		}
		return fmt.Errorf("failed to get BootDependency %s/%s: %w", obj.Namespace, obj.Name, err)
	}

	log.Info("BootDependency found, injecting init containers", "dependencies", len(bd.Spec.DependsOn))

	obj.Spec.Template.Spec.InitContainers = injectInitContainers(
		obj.Spec.Template.Spec.InitContainers,
		bd.Spec.DependsOn,
	)

	return nil
}

// injectInitContainers merges the required wait-for init containers into the
// existing list, skipping any that are already present (idempotent).
func injectInitContainers(existing []corev1.Container, deps []corev1alpha1.ServiceDependency) []corev1.Container {
	existingNames := make(map[string]struct{}, len(existing))
	for _, c := range existing {
		existingNames[c.Name] = struct{}{}
	}

	result := make([]corev1.Container, 0, len(existing)+len(deps))

	// Prepend the wait-for containers so they run before any user-defined init containers.
	for _, dep := range deps {
		name := fmt.Sprintf("wait-for-%s", dep.Service)
		if _, ok := existingNames[name]; ok {
			// Already injected — skip to stay idempotent.
			continue
		}
		result = append(result, buildWaitContainer(name, dep))
	}

	result = append(result, existing...)
	return result
}

// buildWaitContainer creates a busybox init container that polls the given
// service:port via netcat until it is reachable.
func buildWaitContainer(name string, dep corev1alpha1.ServiceDependency) corev1.Container {
	timeout := dep.Timeout
	if timeout == "" {
		timeout = "60s"
	}

	script := fmt.Sprintf(
		"echo 'Waiting for %s:%d...'; "+
			"until nc -z %s %d; do sleep 1; done; "+
			"echo '%s:%d is ready'",
		dep.Service, dep.Port,
		dep.Service, dep.Port,
		dep.Service, dep.Port,
	)

	return corev1.Container{
		Name:            name,
		Image:           "busybox:1.36",
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"sh", "-c", script},
	}
}
