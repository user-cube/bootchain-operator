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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/user-cube/bootchain-operator/api/v1alpha1"
)

var _ = Describe("Deployment Webhook", func() {
	ctx := context.Background()

	Context("When a Deployment has no matching BootDependency", func() {
		It("should not inject any init containers", func() {
			deploy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-match",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "no-match"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "no-match"}},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Name: "app", Image: "nginx"}},
						},
					},
				},
			}

			defaulter := &DeploymentCustomDefaulter{Client: k8sClient}
			Expect(defaulter.Default(ctx, deploy)).To(Succeed())
			Expect(deploy.Spec.Template.Spec.InitContainers).To(BeEmpty())
		})
	})
})

var _ = Describe("buildWaitContainer", func() {
	Context("TCP dependency (no httpPath)", func() {
		It("should generate an nc-based script", func() {
			dep := corev1alpha1.ServiceDependency{
				Service: "my-db",
				Port:    5432,
				Timeout: "30s",
			}
			c := buildWaitContainer("wait-for-my-db", dep)
			Expect(c.Name).To(Equal("wait-for-my-db"))
			Expect(c.Image).To(Equal("busybox:1.36"))
			script := c.Command[len(c.Command)-1]
			Expect(script).To(ContainSubstring("nc -z my-db 5432"))
			Expect(script).To(ContainSubstring("timeout 30s"))
			Expect(script).NotTo(ContainSubstring("wget"))
		})

		It("should default timeout to 60s when unset", func() {
			dep := corev1alpha1.ServiceDependency{Service: "svc", Port: 8080}
			c := buildWaitContainer("wait-for-svc", dep)
			Expect(c.Command[len(c.Command)-1]).To(ContainSubstring("timeout 60s"))
		})
	})

	Context("HTTP dependency (httpPath set)", func() {
		It("should generate a wget-based script for a service dep", func() {
			dep := corev1alpha1.ServiceDependency{
				Service:  "api",
				Port:     8080,
				HTTPPath: "/healthz",
				Timeout:  "45s",
			}
			c := buildWaitContainer("wait-for-api", dep)
			script := c.Command[len(c.Command)-1]
			Expect(script).To(ContainSubstring("wget -q --spider http://api:8080/healthz"))
			Expect(script).To(ContainSubstring("timeout 45s"))
			Expect(script).NotTo(ContainSubstring("nc -z"))
		})

		It("should generate a wget-based script for an external host dep", func() {
			dep := corev1alpha1.ServiceDependency{
				Host:     "db.example.com",
				Port:     5432,
				HTTPPath: "/health",
			}
			c := buildWaitContainer("wait-for-db.example.com", dep)
			script := c.Command[len(c.Command)-1]
			Expect(script).To(ContainSubstring("wget -q --spider http://db.example.com:5432/health"))
			Expect(strings.Count(script, "http://db.example.com:5432/health")).To(Equal(3))
		})
	})
})
