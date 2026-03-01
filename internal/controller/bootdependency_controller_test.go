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
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/user-cube/bootchain-operator/api/v1alpha1"
)

var _ = Describe("BootDependency Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		bootdependency := &corev1alpha1.BootDependency{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind BootDependency")
			err := k8sClient.Get(ctx, typeNamespacedName, bootdependency)
			if err != nil && errors.IsNotFound(err) {
				resource := &corev1alpha1.BootDependency{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: corev1alpha1.BootDependencySpec{
						DependsOn: []corev1alpha1.ServiceDependency{
							{Service: "test-db", Port: 5432},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &corev1alpha1.BootDependency{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("cleaning up the specific resource instance BootDependency")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource without error", func() {
			By("reconciling the created resource")
			controllerReconciler := &BootDependencyReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set resolvedDependencies in status after reconciliation", func() {
			By("reconciling the created resource")
			controllerReconciler := &BootDependencyReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("checking that resolvedDependencies is set in status")
			updated := &corev1alpha1.BootDependency{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())
			Expect(updated.Status.ResolvedDependencies).To(MatchRegexp(`^\d+/\d+$`))
		})
	})

	Context("HTTP health check", func() {
		// parseTestServer extracts host and port from an httptest.Server URL.
		// Works for both http:// and https:// URLs.
		parseTestServer := func(srv *httptest.Server) (string, int32) {
			addr := srv.URL
			addr = strings.TrimPrefix(addr, "https://")
			addr = strings.TrimPrefix(addr, "http://")
			parts := strings.SplitN(addr, ":", 2)
			Expect(parts).To(HaveLen(2))
			p, err := strconv.Atoi(parts[1])
			Expect(err).NotTo(HaveOccurred())
			return parts[0], int32(p)
		}

		createAndReconcile := func(resName string, deps []corev1alpha1.ServiceDependency) *corev1alpha1.BootDependency {
			nn := types.NamespacedName{Name: resName, Namespace: "default"}
			resource := &corev1alpha1.BootDependency{
				ObjectMeta: metav1.ObjectMeta{Name: resName, Namespace: "default"},
				Spec:       corev1alpha1.BootDependencySpec{DependsOn: deps},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			DeferCleanup(func() {
				r := &corev1alpha1.BootDependency{}
				if err := k8sClient.Get(ctx, nn, r); err == nil {
					_ = k8sClient.Delete(ctx, r)
				}
			})

			reconciler := &BootDependencyReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			updated := &corev1alpha1.BootDependency{}
			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			return updated
		}

		It("should count an HTTP dependency as resolved when the endpoint returns 2xx", func() {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			DeferCleanup(srv.Close)
			host, port := parseTestServer(srv)

			updated := createAndReconcile("http-ok-resource", []corev1alpha1.ServiceDependency{
				{Host: host, Port: port, HTTPPath: "/healthz"},
			})
			Expect(updated.Status.ResolvedDependencies).To(Equal("1/1"))
		})

		It("should count an HTTP dependency as not resolved when the endpoint returns 5xx", func() {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			DeferCleanup(srv.Close)
			host, port := parseTestServer(srv)

			updated := createAndReconcile("http-fail-resource", []corev1alpha1.ServiceDependency{
				{Host: host, Port: port, HTTPPath: "/healthz"},
			})
			Expect(updated.Status.ResolvedDependencies).To(Equal("0/1"))
		})

		It("should use the correct HTTP path when probing", func() {
			probed := make(chan string, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case probed <- r.URL.Path:
				default:
				}
				w.WriteHeader(http.StatusOK)
			}))
			DeferCleanup(srv.Close)
			host, port := parseTestServer(srv)

			_ = createAndReconcile("http-path-resource", []corev1alpha1.ServiceDependency{
				{Host: host, Port: port, HTTPPath: "/ready"},
			})
			Expect(<-probed).To(Equal("/ready"))
		})

		It("should resolve service name to FQDN using the BootDependency namespace for HTTP probes", func() {
			// This test guards against the bug where service-based HTTP probes used the bare
			// service name (e.g. "my-svc") instead of the FQDN
			// ("my-svc.<namespace>.svc.cluster.local"), which caused DNS lookup failures when
			// the controller runs in a different namespace than the target service.
			probed := make(chan string, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case probed <- r.Host:
				default:
				}
				w.WriteHeader(http.StatusOK)
			}))
			DeferCleanup(srv.Close)
			_, port := parseTestServer(srv)

			// Use service (not host) — the controller must build the FQDN.
			// We bind the test server on 127.0.0.1, so we override DNS resolution by
			// checking the Host header sent by the HTTP client rather than actual DNS.
			_ = createAndReconcile("http-fqdn-resource", []corev1alpha1.ServiceDependency{
				{Service: "127.0.0.1", Port: port, HTTPPath: "/healthz"},
			})
			// The Host header must contain the FQDN, not the bare service name.
			Expect(<-probed).To(ContainSubstring("svc.cluster.local"))
		})

		It("should resolve an HTTPS dependency when insecure=true and server has a self-signed cert", func() {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			DeferCleanup(srv.Close)
			host, port := parseTestServer(srv)

			updated := createAndReconcile("https-insecure-ok", []corev1alpha1.ServiceDependency{
				{Host: host, Port: port, HTTPPath: "/healthz", HTTPScheme: "https", Insecure: true},
			})
			Expect(updated.Status.ResolvedDependencies).To(Equal("1/1"))
		})

		It("should not resolve an HTTPS dependency when insecure=false and server has a self-signed cert", func() {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			DeferCleanup(srv.Close)
			host, port := parseTestServer(srv)

			updated := createAndReconcile("https-secure-fail", []corev1alpha1.ServiceDependency{
				{Host: host, Port: port, HTTPPath: "/healthz", HTTPScheme: "https", Insecure: false},
			})
			Expect(updated.Status.ResolvedDependencies).To(Equal("0/1"))
		})

		It("should not resolve an HTTPS dependency when the endpoint returns 5xx even with insecure=true", func() {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			DeferCleanup(srv.Close)
			host, port := parseTestServer(srv)

			updated := createAndReconcile("https-insecure-fail", []corev1alpha1.ServiceDependency{
				{Host: host, Port: port, HTTPPath: "/healthz", HTTPScheme: "https", Insecure: true},
			})
			Expect(updated.Status.ResolvedDependencies).To(Equal("0/1"))
		})

		It("should use the specified HTTP method when probing", func() {
			capturedMethod := make(chan string, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case capturedMethod <- r.Method:
				default:
				}
				w.WriteHeader(http.StatusOK)
			}))
			DeferCleanup(srv.Close)
			host, port := parseTestServer(srv)

			_ = createAndReconcile("http-method-resource", []corev1alpha1.ServiceDependency{
				{Host: host, Port: port, HTTPPath: "/healthz", HTTPMethod: "POST"},
			})
			Expect(<-capturedMethod).To(Equal("POST"))
		})

		It("should send custom headers when httpHeaders is set", func() {
			capturedHeader := make(chan string, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case capturedHeader <- r.Header.Get("X-Custom-Header"):
				default:
				}
				w.WriteHeader(http.StatusOK)
			}))
			DeferCleanup(srv.Close)
			host, port := parseTestServer(srv)

			_ = createAndReconcile("http-headers-resource", []corev1alpha1.ServiceDependency{
				{
					Host:     host,
					Port:     port,
					HTTPPath: "/healthz",
					HTTPHeaders: []corev1alpha1.HTTPHeader{
						{Name: "X-Custom-Header", Value: "my-value"},
					},
				},
			})
			Expect(<-capturedHeader).To(Equal("my-value"))
		})

		It("should resolve when response matches an expected status code", func() {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent) // 204
			}))
			DeferCleanup(srv.Close)
			host, port := parseTestServer(srv)

			updated := createAndReconcile("http-status-204-resource", []corev1alpha1.ServiceDependency{
				{Host: host, Port: port, HTTPPath: "/healthz", HTTPExpectedStatuses: []int32{200, 204}},
			})
			Expect(updated.Status.ResolvedDependencies).To(Equal("1/1"))
		})

		It("should not resolve when response does not match any expected status code", func() {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK) // 200 — not in the expected list
			}))
			DeferCleanup(srv.Close)
			host, port := parseTestServer(srv)

			updated := createAndReconcile("http-status-mismatch-resource", []corev1alpha1.ServiceDependency{
				{Host: host, Port: port, HTTPPath: "/healthz", HTTPExpectedStatuses: []int32{204}},
			})
			Expect(updated.Status.ResolvedDependencies).To(Equal("0/1"))
		})

		It("should accept any 2xx when httpExpectedStatuses is omitted", func() {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated) // 201
			}))
			DeferCleanup(srv.Close)
			host, port := parseTestServer(srv)

			updated := createAndReconcile("http-status-2xx-default-resource", []corev1alpha1.ServiceDependency{
				{Host: host, Port: port, HTTPPath: "/healthz"},
			})
			Expect(updated.Status.ResolvedDependencies).To(Equal("1/1"))
		})
	})
})
