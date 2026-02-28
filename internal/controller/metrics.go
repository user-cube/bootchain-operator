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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// reconcileTotal counts reconciliation attempts, labelled by result (success/error).
	reconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bootchain_reconcile_total",
			Help: "Total number of BootDependency reconciliations, partitioned by result.",
		},
		[]string{"result"},
	)

	// reconcileDuration tracks how long each reconciliation takes.
	reconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "bootchain_reconcile_duration_seconds",
			Help:    "Duration of BootDependency reconciliation in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"result"},
	)

	// dependenciesTotal tracks the total number of declared dependencies per BootDependency.
	dependenciesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bootchain_dependencies_total",
			Help: "Total number of declared dependencies for a BootDependency resource.",
		},
		[]string{"namespace", "name"},
	)

	// dependenciesReady tracks how many dependencies are currently reachable per BootDependency.
	dependenciesReady = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bootchain_dependencies_ready",
			Help: "Number of dependencies currently reachable for a BootDependency resource.",
		},
		[]string{"namespace", "name"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		reconcileTotal,
		reconcileDuration,
		dependenciesTotal,
		dependenciesReady,
	)
}
