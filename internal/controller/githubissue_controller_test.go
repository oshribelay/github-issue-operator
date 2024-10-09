/*
Copyright 2024.

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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	issuev1 "github.com/oshribelay/github-issue-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"time"
)

var _ = Describe("GithubIssue Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "test-resource"
			timeout      = time.Second * 10
			interval     = time.Second * 1
		)
		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		githubIssue := &issuev1.GithubIssue{}
		secret := &corev1.Secret{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind GithubIssue")
			// Delete any existing resources first to ensure clean state
			k8sClient.Delete(ctx, githubIssue)
			k8sClient.Delete(ctx, secret)

			// create issue if it doesn't exist
			err := k8sClient.Get(ctx, typeNamespacedName, githubIssue)
			if err != nil && errors.IsNotFound(err) {
				resource := &issuev1.GithubIssue{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: issuev1.GithubIssueSpec{
						Repo:        os.Getenv("TEST_REPO_URL"),
						Title:       "Test Issue",
						Description: "This is a test issue",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
			By("creating the Secret with OwnerReference")
			// Wait for the GithubIssue to be created before creating the secret
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, githubIssue)
			}, timeout, interval).Should(Succeed())

			// create Secret with OwnerReference to the GithubIssue
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-token-secret", typeNamespacedName.Name),
					Namespace: typeNamespacedName.Namespace,
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(githubIssue, schema.GroupVersionKind{
						Group:   "issue.core.github.io",
						Version: "v1",
						Kind:    "GithubIssue",
					})}},
				StringData: map[string]string{
					"token": os.Getenv("TEST_AUTH_TOKEN"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			// ensure the Secret has been created
			createdSecret := &corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(
					ctx,
					types.NamespacedName{
						Namespace: secret.Namespace,
						Name:      secret.Name,
					},
					createdSecret,
				)
				return err == nil
			}).Should(BeTrue())

			Expect(secret.OwnerReferences).ToNot(BeEmpty(), "secret should have an OwnerReference")
			Expect(secret.OwnerReferences[0].Name).To(Equal(resourceName), "secret should be owned by the GithubIssue")
		})
		AfterEach(func() {
			By("cleaning up the GithubIssue resource")
			// get the latest version of the resource
			updatedIssue := &issuev1.GithubIssue{}
			err := k8sClient.Get(ctx, typeNamespacedName, updatedIssue)
			if err == nil {
				By("deleting the GithubIssue")
				Expect(k8sClient.Delete(ctx, updatedIssue)).To(Succeed())

				By("waiting for the controller to handle deletion")
				Eventually(func() string {
					err := k8sClient.Get(ctx, typeNamespacedName, updatedIssue)
					if errors.IsNotFound(err) {
						return "deleted"
					}
					if err != nil {
						return fmt.Sprintf("error checking resource: %v", err)
					}
					return fmt.Sprintf("resource still exists with finalizers: %v", updatedIssue.Finalizers)
				}, timeout, interval).Should(Equal("deleted"), "Resource should be fully deleted by the controller")

				// the secret should be automatically deleted due to OwnerReference
				By("deleting the Secret")
				Expect(k8sClient.Delete(ctx, secret)).To(Succeed())

				By("waiting for the controller to handle deletion")
				Eventually(func() string {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Namespace: secret.Namespace,
						Name:      secret.Name,
					}, secret)
					if errors.IsNotFound(err) {
						return "deleted"
					}
					if err != nil {
						return fmt.Sprintf("error checking secret: %v", err)
					}
					return fmt.Sprintf("resource still exists with OwnerRefernces: %v", secret.OwnerReferences)

				}, timeout, interval).Should(Equal("deleted"), "Secret should be deleted")
			}
		})
		It("Should create the issue if it doesn't exist", func() {
			By("verifying the GithubIssue resource exists")
			createdIssue := &issuev1.GithubIssue{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, createdIssue)
			}, timeout, interval).Should(Succeed())

			By("verifying the secret exists with valid token")
			Eventually(func() bool {
				createdSecret := &corev1.Secret{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: secret.Namespace,
					Name:      secret.Name,
				}, createdSecret)
				if err != nil {
					return false
				}
				token, exists := createdSecret.Data["token"]
				return exists && len(token) > 0
			}, timeout, interval).Should(BeTrue(), "Secret should exist with valid token")

			By("verifying the GitHub issue is created and status is updated")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, createdIssue)
				if err != nil {
					return false
				}
				// Check if the status has been updated with an issue number
				return createdIssue.Status.IssueNumber > 0
			}, timeout, interval).Should(BeTrue(), "GitHub issue should be created and status updated")

			By("verifying the issue details are correct")
			Expect(createdIssue.Spec.Title).To(Equal("Test Issue"))
			Expect(createdIssue.Spec.Description).To(Equal("This is a test issue"))
			Expect(createdIssue.Spec.Repo).To(Equal("https://github.com/oshribelay/test-issues-operator"))
		})
	})
})
