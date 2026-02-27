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
	"fmt"
	"net"
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
)

// BootDependencyReconciler reconciles a BootDependency object
type BootDependencyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=core.bootchain.ruicoelho.dev,resources=bootdependencies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.bootchain.ruicoelho.dev,resources=bootdependencies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.bootchain.ruicoelho.dev,resources=bootdependencies/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

func (r *BootDependencyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var bd corev1alpha1.BootDependency
	if err := r.Get(ctx, req.NamespacedName, &bd); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	resolved := 0
	total := len(bd.Spec.DependsOn)
	allReady := true

	for _, dep := range bd.Spec.DependsOn {
		addr := fmt.Sprintf("%s.%s.svc.cluster.local:%d", dep.Service, req.Namespace, dep.Port)
		conn, err := net.DialTimeout("tcp", addr, dialTimeout)
		if err != nil {
			log.Info("Dependency not reachable", "service", dep.Service, "port", dep.Port, "error", err)
			r.Recorder.Eventf(&bd, corev1.EventTypeWarning, "DependencyNotReady",
				"Service %s:%d is not reachable", dep.Service, dep.Port)
			allReady = false
			continue
		}
		conn.Close()
		resolved++
		log.Info("Dependency reachable", "service", dep.Service, "port", dep.Port)
	}

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
		return ctrl.Result{}, err
	}

	if allReady {
		r.Recorder.Eventf(&bd, corev1.EventTypeNormal, "AllDependenciesReady",
			"All %d dependencies are reachable", total)
		return ctrl.Result{RequeueAfter: requeueAfterReady}, nil
	}

	return ctrl.Result{RequeueAfter: requeueAfterNotReady}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BootDependencyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.BootDependency{}).
		Named("bootdependency").
		Complete(r)
}
