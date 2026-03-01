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
			Expect(c.Image).To(Equal("ghcr.io/user-cube/bootchain-operator/minimal-tools:latest"))
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
			Expect(script).NotTo(ContainSubstring("--no-check-certificate"))
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

	Context("HTTPS dependency (httpScheme=https)", func() {
		It("should use https scheme and no --no-check-certificate when insecure is false", func() {
			dep := corev1alpha1.ServiceDependency{
				Service:    "secure-api",
				Port:       443,
				HTTPPath:   "/healthz",
				HTTPScheme: "https",
			}
			c := buildWaitContainer("wait-for-secure-api", dep)
			script := c.Command[len(c.Command)-1]
			Expect(script).To(ContainSubstring("wget -q --spider https://secure-api:443/healthz"))
			Expect(script).NotTo(ContainSubstring("--no-check-certificate"))
			Expect(script).NotTo(ContainSubstring("http://"))
		})

		It("should add --no-check-certificate when insecure is true", func() {
			dep := corev1alpha1.ServiceDependency{
				Service:    "self-signed-api",
				Port:       8443,
				HTTPPath:   "/ready",
				HTTPScheme: "https",
				Insecure:   true,
			}
			c := buildWaitContainer("wait-for-self-signed-api", dep)
			script := c.Command[len(c.Command)-1]
			Expect(script).To(ContainSubstring("wget -q --spider --no-check-certificate https://self-signed-api:8443/ready"))
		})

		It("should support https with an external host", func() {
			dep := corev1alpha1.ServiceDependency{
				Host:       "api.example.com",
				Port:       443,
				HTTPPath:   "/health",
				HTTPScheme: "https",
				Insecure:   true,
			}
			c := buildWaitContainer("wait-for-api.example.com", dep)
			script := c.Command[len(c.Command)-1]
			Expect(script).To(ContainSubstring("wget -q --spider --no-check-certificate https://api.example.com:443/health"))
			Expect(strings.Count(script, "https://api.example.com:443/health")).To(Equal(3))
		})
	})

	Context("Advanced HTTP fields (curl path)", func() {
		It("should use curl and the specified method when httpMethod is set", func() {
			dep := corev1alpha1.ServiceDependency{
				Service:    "api",
				Port:       8080,
				HTTPPath:   "/healthz",
				HTTPMethod: "POST",
				Timeout:    "30s",
			}
			c := buildWaitContainer("wait-for-api", dep)
			script := c.Command[len(c.Command)-1]
			Expect(script).To(ContainSubstring("curl"))
			Expect(script).To(ContainSubstring("-X POST"))
			Expect(script).NotTo(ContainSubstring("wget"))
		})

		It("should include --header flags for each entry in httpHeaders", func() {
			dep := corev1alpha1.ServiceDependency{
				Service:  "api",
				Port:     8080,
				HTTPPath: "/healthz",
				HTTPHeaders: []corev1alpha1.HTTPHeader{
					{Name: "Authorization", Value: "Bearer token123"},
					{Name: "X-Trace-Id", Value: "abc"},
				},
			}
			c := buildWaitContainer("wait-for-api", dep)
			script := c.Command[len(c.Command)-1]
			Expect(script).To(ContainSubstring("curl"))
			Expect(script).To(ContainSubstring("--header 'Authorization: Bearer token123'"))
			Expect(script).To(ContainSubstring("--header 'X-Trace-Id: abc'"))
		})

		It("should use -k flag (not --no-check-certificate) when insecure=true in curl mode", func() {
			dep := corev1alpha1.ServiceDependency{
				Service:    "api",
				Port:       443,
				HTTPPath:   "/healthz",
				HTTPScheme: "https",
				Insecure:   true,
				HTTPMethod: "HEAD",
			}
			c := buildWaitContainer("wait-for-api", dep)
			script := c.Command[len(c.Command)-1]
			Expect(script).To(ContainSubstring("curl"))
			Expect(script).To(ContainSubstring(" -k"))
			Expect(script).NotTo(ContainSubstring("--no-check-certificate"))
		})

		It("should use case pattern for explicit expected statuses", func() {
			dep := corev1alpha1.ServiceDependency{
				Service:              "api",
				Port:                 8080,
				HTTPPath:             "/healthz",
				HTTPExpectedStatuses: []int32{200, 204},
			}
			c := buildWaitContainer("wait-for-api", dep)
			script := c.Command[len(c.Command)-1]
			Expect(script).To(ContainSubstring("curl"))
			Expect(script).To(ContainSubstring("200|204"))
		})

		It("should use arithmetic 2xx check when curl is needed but no explicit statuses are set", func() {
			dep := corev1alpha1.ServiceDependency{
				Service:    "api",
				Port:       8080,
				HTTPPath:   "/healthz",
				HTTPMethod: "HEAD",
			}
			c := buildWaitContainer("wait-for-api", dep)
			script := c.Command[len(c.Command)-1]
			Expect(script).To(ContainSubstring(`[ "$STATUS" -ge 200 ]`))
			Expect(script).To(ContainSubstring(`[ "$STATUS" -lt 300 ]`))
		})

		It("should keep wget path when none of the advanced fields are set", func() {
			dep := corev1alpha1.ServiceDependency{
				Service:  "api",
				Port:     8080,
				HTTPPath: "/healthz",
			}
			c := buildWaitContainer("wait-for-api", dep)
			script := c.Command[len(c.Command)-1]
			Expect(script).To(ContainSubstring("wget"))
			Expect(script).NotTo(ContainSubstring("curl"))
		})
	})
})
