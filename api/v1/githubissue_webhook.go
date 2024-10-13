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

package v1

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"net/url"
	"regexp"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var githubissuelog = logf.Log.WithName("githubissue-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *GithubIssue) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-issue-core-github-io-v1-githubissue,mutating=true,failurePolicy=fail,sideEffects=None,groups=issue.core.github.io,resources=githubissues,verbs=create;update,versions=v1,name=mgithubissue.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &GithubIssue{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *GithubIssue) Default() {
	githubissuelog.Info("default", "name", r.Name)
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-issue-core-github-io-v1-githubissue,mutating=false,failurePolicy=fail,sideEffects=None,groups=issue.core.github.io,resources=githubissues,verbs=create;update,versions=v1,name=vgithubissue.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &GithubIssue{}

// isValidRepoUrl validates the GitHub repository URL format.
func validateRepoURL(repoUrl string) *field.Error {
	fldPath := field.NewPath("spec").Child("repo")
	// check if it is a proper URL
	parsedURL, err := url.Parse(repoUrl)
	if err != nil {
		return field.Invalid(fldPath, repoUrl, "Invalid url format")
	}
	// check if it is https
	if parsedURL.Scheme != "https" {
		return field.Invalid(fldPath, repoUrl, "repository url should start with https")
	}
	// ensure the host is actually GitHub
	if parsedURL.Host != "github.com" {
		return field.Invalid(fldPath, repoUrl, "the host name of the repository should be github.com")
	}
	// user regex to check if the url is in the right format
	re := regexp.MustCompile(`^/[^/]+/[^/]+$`)
	if !re.MatchString(parsedURL.Path) {
		return field.Invalid(fldPath, repoUrl, "repository URL must be in the format 'https://github.com/{owner}/{repo}'")
	}

	return nil
}

// validateTitle checks if the title is not empty
func validateTitle(title string) *field.Error {
	if len(title) < 1 {
		return field.Invalid(field.NewPath("spec").Child("title"), title, "title must not be empty")
	}
	return nil
}

func validateDescription(description string) *field.Error {
	if len(description) > 256 {
		return field.Invalid(field.NewPath("spec").Child("description"), description, "description must not be longer than 256 characters")
	}
	return nil
}

func validateGithubIssue(githubIssue *GithubIssue) error {
	var allErrs field.ErrorList
	if err := validateTitle(githubIssue.Spec.Title); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := validateDescription(githubIssue.Spec.Description); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := validateRepoURL(githubIssue.Spec.Repo); err != nil {
		allErrs = append(allErrs, err)
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: GroupVersion.Group, Kind: "GithubIssue"},
		githubIssue.Name,
		allErrs,
	)
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *GithubIssue) ValidateCreate() (admission.Warnings, error) {
	githubissuelog.Info("validate create", "name", r.Name)

	if err := validateGithubIssue(r); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *GithubIssue) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	githubissuelog.Info("validate update", "name", r.Name)

	if err := validateGithubIssue(r); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *GithubIssue) ValidateDelete() (admission.Warnings, error) {
	githubissuelog.Info("validate delete", "name", r.Name)

	return nil, nil
}
