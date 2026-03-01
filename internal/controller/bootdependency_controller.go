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

package controller

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	corev1alpha1 "github.com/user-cube/bootchain-operator/api/v1alpha1"
)

const (
	conditionReady       = "Ready"
	requeueAfterReady    = 30 * time.Second
	requeueAfterNotReady = 10 * time.Second
	dialTimeout          = 3 * time.Second
	httpTimeout          = 3 * time.Second
)

// BootDependencyReconciler reconciles a BootDependency object
type BootDependencyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=core.bootchain-operator.ruicoelho.dev,resources=bootdependencies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.bootchain-operator.ruicoelho.dev,resources=bootdependencies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.bootchain-operator.ruicoelho.dev,resources=bootdependencies/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

func (r *BootDependencyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	start := time.Now()

	var bd corev1alpha1.BootDependency
	if err := r.Get(ctx, req.NamespacedName, &bd); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	resolved := 0
	total := len(bd.Spec.DependsOn)
	allReady := true

	secureClient := &http.Client{Timeout: httpTimeout}
	insecureClient := &http.Client{
		Timeout: httpTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
	for _, dep := range bd.Spec.DependsOn {
		label := depLabel(dep)
		var checkErr error
		if dep.HTTPPath != "" {
			scheme := dep.HTTPScheme
			if scheme == "" {
				scheme = "http"
			}
			url := fmt.Sprintf("%s://%s:%d%s", scheme, depHost(dep, bd.Namespace), dep.Port, dep.HTTPPath)
			httpClient := secureClient
			if dep.Insecure {
				httpClient = insecureClient
			}
			method := dep.HTTPMethod
			if method == "" {
				method = http.MethodGet
			}
			req, reqErr := http.NewRequest(method, url, nil) //nolint:noctx
			if reqErr != nil {
				checkErr = reqErr
			} else {
				for _, h := range dep.HTTPHeaders {
					req.Header.Set(h.Name, h.Value)
				}
				resp, err := httpClient.Do(req)
				if err != nil {
					checkErr = err
				} else {
					_ = resp.Body.Close()
					if !statusAccepted(resp.StatusCode, dep.HTTPExpectedStatuses) {
						checkErr = fmt.Errorf("HTTP %d", resp.StatusCode)
					}
				}
			}
		} else {
			addr := depAddress(dep, req.Namespace)
			conn, err := net.DialTimeout("tcp", addr, dialTimeout)
			if err != nil {
				checkErr = err
			} else {
				_ = conn.Close()
			}
		}

		if checkErr != nil {
			log.Info("Dependency not reachable", "dependency", label, "port", dep.Port, "error", checkErr)
			r.Recorder.Eventf(&bd, corev1.EventTypeWarning, "DependencyNotReady",
				"Dependency %s:%d is not reachable", label, dep.Port)
			allReady = false
			continue
		}
		resolved++
		log.Info("Dependency reachable", "dependency", label, "port", dep.Port)
	}

	dependenciesTotal.WithLabelValues(bd.Namespace, bd.Name).Set(float64(total))
	dependenciesReady.WithLabelValues(bd.Namespace, bd.Name).Set(float64(resolved))

	patch := client.MergeFrom(bd.DeepCopy())
	bd.Status.ResolvedDependencies = fmt.Sprintf("%d/%d", resolved, total)

	var condStatus metav1.ConditionStatus
	var reason, message string
	if allReady {
		condStatus = metav1.ConditionTrue
		reason = "AllDependenciesReady"
		message = fmt.Sprintf("All %d dependencies are reachable", total)
	} else {
		condStatus = metav1.ConditionFalse
		reason = "DependenciesNotReady"
		message = fmt.Sprintf("%d/%d dependencies are reachable", resolved, total)
	}

	meta.SetStatusCondition(&bd.Status.Conditions, metav1.Condition{
		Type:               conditionReady,
		Status:             condStatus,
		ObservedGeneration: bd.Generation,
		Reason:             reason,
		Message:            message,
	})

	if err := r.Status().Patch(ctx, &bd, patch); err != nil {
		log.Error(err, "Failed to patch status")
		reconcileTotal.WithLabelValues("error").Inc()
		reconcileDuration.WithLabelValues("error").Observe(time.Since(start).Seconds())
		return ctrl.Result{}, err
	}

	reconcileTotal.WithLabelValues("success").Inc()
	reconcileDuration.WithLabelValues("success").Observe(time.Since(start).Seconds())

	if allReady {
		r.Recorder.Eventf(&bd, corev1.EventTypeNormal, "AllDependenciesReady",
			"All %d dependencies are reachable", total)
		return ctrl.Result{RequeueAfter: requeueAfterReady}, nil
	}

	return ctrl.Result{RequeueAfter: requeueAfterNotReady}, nil
}

// depAddress returns the dial address (host:port) for a dependency.
// For in-cluster services it resolves to the FQDN; for external hosts it uses the host directly.
func depAddress(dep corev1alpha1.ServiceDependency, namespace string) string {
	return fmt.Sprintf("%s:%d", depHost(dep, namespace), dep.Port)
}

// depHost returns the hostname for a dependency.
// For in-cluster services it builds the FQDN <service>.<namespace>.svc.cluster.local so that
// the controller — which runs in a different namespace — can always resolve the service correctly.
// For external dependencies (host field set) it returns the host directly.
func depHost(dep corev1alpha1.ServiceDependency, namespace string) string {
	if dep.Host != "" {
		return dep.Host
	}
	return fmt.Sprintf("%s.%s.svc.cluster.local", dep.Service, namespace)
}

// statusAccepted returns true when code is in the accepted list.
// When the list is empty it falls back to the 2xx range (200–299).
func statusAccepted(code int, accepted []int32) bool {
	if len(accepted) == 0 {
		return code >= 200 && code < 300
	}
	return slices.Contains(accepted, int32(code))
}

// depLabel returns a human-readable identifier for a dependency (for logs and events).
func depLabel(dep corev1alpha1.ServiceDependency) string {
	if dep.Host != "" {
		return dep.Host
	}
	return dep.Service
}

// SetupWithManager sets up the controller with the Manager.
func (r *BootDependencyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.BootDependency{}).
		Named("bootdependency").
		Complete(r)
}
