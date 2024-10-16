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
	"github.com/google/go-github/v47/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	issuev1 "github.com/oshribelay/github-issue-operator/api/v1"
	"github.com/oshribelay/github-issue-operator/internal/controller/utils"
	"golang.org/x/oauth2"
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

			// Ensure the Secret is deleted before creating it again
			err = k8sClient.Delete(context.TODO(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-resource-token-secret",
					Namespace: "default",
				},
			})
			// Only return an error if the Secret exists and can't be deleted
			Expect(err != nil && !errors.IsNotFound(err)).To(BeFalse())

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
			Expect(createdIssue.Spec.Repo).To(Equal(os.Getenv("TEST_REPO_URL")))
		})
		It("Should close the issue on delete", func() {
			By("creating new github client")
			ts := oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: os.Getenv("TEST_AUTH_TOKEN")},
			)

			tc := oauth2.NewClient(context.Background(), ts)
			client := github.NewClient(tc)
			Expect(client).ToNot(BeNil())

			By("waiting for the issue number to be set")
			createdIssue := &issuev1.GithubIssue{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, createdIssue)
				return err == nil && createdIssue.Status.IssueNumber > 0
			}, timeout, interval).Should(BeTrue(), "Issue number should be set in the status")

			By("deleting the issue")
			Expect(k8sClient.Delete(ctx, createdIssue)).To(Succeed())

			By("waiting for the controller to handle deletion")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, createdIssue)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("waiting for the issue to be deleted on GitHub")
			Eventually(func() bool {
				isIssueClosed, err := checkIssueClosedOnGithub(client, createdIssue, ctx)
				if err != nil {
					fmt.Println(err)
					return false
				}
				return isIssueClosed
			}, timeout, interval).Should(BeTrue(), "Issue should be closed on GitHub")
		})
		It("Should enforce webhook validation during creation", func() {
			CreationTestCases := []struct {
				name          string
				issueSpec     issuev1.GithubIssueSpec
				expectedError string
			}{
				{
					name: "valid-creation",
					issueSpec: issuev1.GithubIssueSpec{
						Repo:        os.Getenv("TEST_REPO_URL"),
						Title:       "valid creation title",
						Description: "this is a valid issue",
					},
					expectedError: "",
				},
				{
					name: "invalid-title",
					issueSpec: issuev1.GithubIssueSpec{
						Repo:        os.Getenv("TEST_REPO_URL"),
						Title:       "",
						Description: "test description",
					},
					expectedError: "title must not be empty",
				},
				{
					name: "invalid-repo-url",
					issueSpec: issuev1.GithubIssueSpec{
						Repo:        "invalid-url",
						Title:       "Test Title",
						Description: "Test description",
					},
					expectedError: "repository url should start with https",
				},
			}
			for _, tc := range CreationTestCases {
				tc := tc
				By(fmt.Sprintf("now testing %s", tc.name))

				issue := &issuev1.GithubIssue{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-%s", resourceName, tc.name),
						Namespace: "default",
					},
					Spec: tc.issueSpec,
				}

				// clean up any existing issues
				k8sClient.Delete(ctx, issue)

				// attempt to create
				err := k8sClient.Create(ctx, issue)

				if tc.expectedError == "" {
					Expect(err).NotTo(HaveOccurred(), "valid creation should succeed")
					// clean up after successful creation
					Expect(k8sClient.Delete(ctx, issue)).To(Succeed())
				} else {
					Expect(err).To(HaveOccurred(), "invalid creation should fail")
					Expect(err.Error()).To(ContainSubstring(tc.expectedError))
				}
			}
		})
		It("Should enforce webhook validation during update", func() {
			const (
				validTitle       = "Initial Valid Title"
				validDescription = "Initial Valid Description"
			)

			ctx := context.Background()

			// Create a valid initial resource
			initialIssue := &issuev1.GithubIssue{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-update-test", resourceName),
					Namespace: "default",
				},
				Spec: issuev1.GithubIssueSpec{
					Repo:        os.Getenv("TEST_REPO_URL"),
					Title:       validTitle,
					Description: validDescription,
				},
			}

			// Clean up any existing test resources
			k8sClient.Delete(ctx, initialIssue)

			// Create the initial valid resource
			Expect(k8sClient.Create(ctx, initialIssue)).To(Succeed(), "Should create initial valid resource")

			// Wait for the resource to be created
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      initialIssue.Name,
					Namespace: initialIssue.Namespace,
				}, initialIssue)
			}, timeout, interval).Should(Succeed())

			UpdateTestCases := []struct {
				name          string
				updateSpec    issuev1.GithubIssueSpec
				expectedError string
			}{
				{
					name: "invalid-title-update",
					updateSpec: issuev1.GithubIssueSpec{
						Repo:        os.Getenv("TEST_REPO_URL"),
						Title:       "",
						Description: "test description",
					},
					expectedError: "title must not be empty",
				},
				{
					name: "invalid-repo-url-update",
					updateSpec: issuev1.GithubIssueSpec{
						Repo:        "invalid-url",
						Title:       "Test Title",
						Description: "Test description",
					},
					expectedError: "repository url should start with https",
				},
			}

			for _, tc := range UpdateTestCases {
				tc := tc // Capture range variable
				By(fmt.Sprintf("testing update case: %s", tc.name))

				// Get the latest version of the resource
				updatedIssue := &issuev1.GithubIssue{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      initialIssue.Name,
					Namespace: initialIssue.Namespace,
				}, updatedIssue)).To(Succeed())

				// attempt to update
				updatedIssue.Spec = tc.updateSpec
				err := k8sClient.Update(ctx, updatedIssue)

				Expect(err).To(HaveOccurred(), "Invalid update should fail")
				Expect(err.Error()).To(ContainSubstring(tc.expectedError))

				// Verify the resource wasn't changed
				currentIssue := &issuev1.GithubIssue{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      initialIssue.Name,
					Namespace: initialIssue.Namespace,
				}, currentIssue)).To(Succeed())

				// Verify the spec wasn't changed
				Expect(currentIssue.Spec.Title).To(Equal(validTitle), "Title should remain unchanged")
				Expect(currentIssue.Spec.Repo).To(Equal(os.Getenv("TEST_REPO_URL")), "Repo should remain unchanged")
				Expect(currentIssue.Spec.Description).To(Equal(validDescription), "Description should remain unchanged")
			}
		})
	})
})

func checkIssueClosedOnGithub(c *github.Client, githubIssue *issuev1.GithubIssue, ctx context.Context) (bool, error) {
	owner, repo, err := utils.ParseRepoUrl(githubIssue.Spec.Repo)
	if err != nil {
		return false, err
	}
	issue, _, err := c.Issues.Get(ctx, owner, repo, int(githubIssue.Status.IssueNumber))
	if err != nil {
		return false, err
	}

	if *issue.State == "closed" {
		return true, nil
	}
	return false, nil
}
