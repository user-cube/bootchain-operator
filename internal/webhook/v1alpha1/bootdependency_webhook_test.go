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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/user-cube/bootchain-operator/api/v1alpha1"
)

var _ = Describe("BootDependency Webhook", func() {
	ctx := context.Background()

	Context("When creating a BootDependency with no circular dependencies", func() {
		It("should allow creation", func() {
			bd := &corev1alpha1.BootDependency{
				ObjectMeta: metav1.ObjectMeta{Name: "svc-a", Namespace: "default"},
				Spec: corev1alpha1.BootDependencySpec{
					DependsOn: []corev1alpha1.ServiceDependency{
						{Service: "postgres", Port: 5432},
					},
				},
			}
			validator := &BootDependencyCustomValidator{Client: k8sClient}
			warnings, err := validator.ValidateCreate(ctx, bd)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})

	Context("When creating a BootDependency with both service and host set", func() {
		It("should deny creation when both service and host are specified", func() {
			bd := &corev1alpha1.BootDependency{
				ObjectMeta: metav1.ObjectMeta{Name: "svc-invalid", Namespace: "default"},
				Spec: corev1alpha1.BootDependencySpec{
					DependsOn: []corev1alpha1.ServiceDependency{
						{Service: "postgres", Host: "db.example.com", Port: 5432},
					},
				},
			}
			validator := &BootDependencyCustomValidator{Client: k8sClient}
			_, err := validator.ValidateCreate(ctx, bd)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("exactly one of service or host must be specified"))
		})
	})

	Context("When creating a BootDependency with neither service nor host set", func() {
		It("should deny creation when neither service nor host is specified", func() {
			bd := &corev1alpha1.BootDependency{
				ObjectMeta: metav1.ObjectMeta{Name: "svc-empty", Namespace: "default"},
				Spec: corev1alpha1.BootDependencySpec{
					DependsOn: []corev1alpha1.ServiceDependency{
						{Port: 5432},
					},
				},
			}
			validator := &BootDependencyCustomValidator{Client: k8sClient}
			_, err := validator.ValidateCreate(ctx, bd)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("exactly one of service or host must be specified"))
		})
	})

	Context("When creating a BootDependency with only a host set", func() {
		It("should allow creation when only host is specified", func() {
			bd := &corev1alpha1.BootDependency{
				ObjectMeta: metav1.ObjectMeta{Name: "svc-external", Namespace: "default"},
				Spec: corev1alpha1.BootDependencySpec{
					DependsOn: []corev1alpha1.ServiceDependency{
						{Host: "db.example.com", Port: 5432},
					},
				},
			}
			validator := &BootDependencyCustomValidator{Client: k8sClient}
			warnings, err := validator.ValidateCreate(ctx, bd)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})

	Context("When creating a BootDependency with valid HTTPS configuration", func() {
		It("should allow creation with httpScheme and httpPath set together", func() {
			bd := &corev1alpha1.BootDependency{
				ObjectMeta: metav1.ObjectMeta{Name: "svc-https-valid", Namespace: "default"},
				Spec: corev1alpha1.BootDependencySpec{
					DependsOn: []corev1alpha1.ServiceDependency{
						{Host: "api.example.com", Port: 443, HTTPPath: "/healthz", HTTPScheme: "https"},
					},
				},
			}
			validator := &BootDependencyCustomValidator{Client: k8sClient}
			warnings, err := validator.ValidateCreate(ctx, bd)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("should allow creation with insecure=true when httpPath is also set", func() {
			bd := &corev1alpha1.BootDependency{
				ObjectMeta: metav1.ObjectMeta{Name: "svc-https-insecure-valid", Namespace: "default"},
				Spec: corev1alpha1.BootDependencySpec{
					DependsOn: []corev1alpha1.ServiceDependency{
						{Host: "api.example.com", Port: 443, HTTPPath: "/healthz", HTTPScheme: "https", Insecure: true},
					},
				},
			}
			validator := &BootDependencyCustomValidator{Client: k8sClient}
			warnings, err := validator.ValidateCreate(ctx, bd)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})

	Context("CEL validation: httpScheme and insecure require httpPath", func() {
		It("should reject httpScheme set without httpPath (via API server)", func() {
			bd := &corev1alpha1.BootDependency{
				ObjectMeta: metav1.ObjectMeta{Name: "svc-scheme-no-path", Namespace: "default"},
				Spec: corev1alpha1.BootDependencySpec{
					DependsOn: []corev1alpha1.ServiceDependency{
						{Service: "api", Port: 8080, HTTPScheme: "https"},
					},
				},
			}
			err := k8sClient.Create(ctx, bd)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("httpScheme requires httpPath to be set"))
		})

		It("should reject insecure=true without httpPath (via API server)", func() {
			bd := &corev1alpha1.BootDependency{
				ObjectMeta: metav1.ObjectMeta{Name: "svc-insecure-no-path", Namespace: "default"},
				Spec: corev1alpha1.BootDependencySpec{
					DependsOn: []corev1alpha1.ServiceDependency{
						{Service: "api", Port: 8080, Insecure: true},
					},
				},
			}
			err := k8sClient.Create(ctx, bd)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("insecure requires httpPath to be set"))
		})
	})

	Context("When creating a BootDependency that introduces a circular dependency", func() {
		BeforeEach(func() {
			bdB := &corev1alpha1.BootDependency{
				ObjectMeta: metav1.ObjectMeta{Name: "svc-b", Namespace: "default"},
				Spec: corev1alpha1.BootDependencySpec{
					DependsOn: []corev1alpha1.ServiceDependency{
						{Service: "svc-c", Port: 8080},
					},
				},
			}
			Expect(k8sClient.Create(ctx, bdB)).To(Succeed())
		})

		AfterEach(func() {
			bd := &corev1alpha1.BootDependency{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "svc-b", Namespace: "default"}, bd)
			if err == nil {
				_ = k8sClient.Delete(ctx, bd)
			}
		})

		It("should deny creation when svc-c depends on svc-b (cycle: svc-c → svc-b → svc-c)", func() {
			bdC := &corev1alpha1.BootDependency{
				ObjectMeta: metav1.ObjectMeta{Name: "svc-c", Namespace: "default"},
				Spec: corev1alpha1.BootDependencySpec{
					DependsOn: []corev1alpha1.ServiceDependency{
						{Service: "svc-b", Port: 8080},
					},
				},
			}
			validator := &BootDependencyCustomValidator{Client: k8sClient}
			_, err := validator.ValidateCreate(ctx, bdC)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("circular dependency"))
		})
	})
})
