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
	"github.com/oshribelay/github-issue-operator/internal/controller/resources"
	"github.com/oshribelay/github-issue-operator/internal/controller/status"
	"github.com/oshribelay/github-issue-operator/internal/controller/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	issuev1 "github.com/oshribelay/github-issue-operator/api/v1"
)

// GithubIssueReconciler reconciles a GithubIssue object
type GithubIssueReconciler struct {
	Client       client.Client
	GithubClient *resources.GithubClient
	Scheme       *runtime.Scheme
}

// NewGithubIssueReconciler initializes a new GithubIssueReconciler
func NewGithubIssueReconciler(client client.Client, scheme *runtime.Scheme, token string) *GithubIssueReconciler {
	return &GithubIssueReconciler{
		Client:       client,
		GithubClient: resources.NewGithubClient(token), // Initialize the GithubClient
		Scheme:       scheme,
	}
}

// +kubebuilder:rbac:groups=issue.core.github.io,resources=githubissues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=issue.core.github.io,resources=githubissues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=issue.core.github.io,resources=githubissues/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *GithubIssueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Make sure to check if r.GithubClient is nil before using it
	if r.GithubClient == nil {
		log.V(1).Info("GithubClient is not initialized")
		return reconcile.Result{}, fmt.Errorf("GithubClient is not initialized")
	}

	// Fetch the GithubIssue custom resource
	githubIssue := &issuev1.GithubIssue{}
	if err := r.Client.Get(ctx, req.NamespacedName, githubIssue); err != nil {
		log.Error(err, "unable to fetch GithubIssue")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	owner, repo, err := utils.ParseRepoUrl(githubIssue.Spec.Repo)
	if err != nil {
		log.Error(err, "unable to parse repo url")
		return ctrl.Result{}, err
	}
	title := githubIssue.Spec.Title
	description := githubIssue.Spec.Description

	// check if issue exists
	issue, err := r.GithubClient.CheckIssueExists(owner, repo, title)
	if err != nil {
		log.Error(err, "unable to check issue existence")
		return ctrl.Result{}, err
	}

	if issue == nil {
		// create issue if it doesn't exist
		issue, err = r.GithubClient.CreateIssue(owner, repo, title, description)
		if err != nil {
			log.Error(err, "unable to create issue")
			return ctrl.Result{}, err
		}
	} else {
		// update the issue if it exists
		updatedIssue, err := r.GithubClient.UpdateIssue(owner, repo, issue, description)
		if err != nil {
			log.Error(err, "unable to update issue")
			return ctrl.Result{}, err
		}
		issue = updatedIssue
	}

	// update the status of the GithubIssue CR
	if err := status.Update(ctx, r.Client, githubIssue, issue); err != nil {
		log.Error(err, "unable to update GithubIssue")
		return ctrl.Result{}, err
	}

	if *issue.State == "open" {
		// if the issue is still open requeue
		return ctrl.Result{Requeue: true}, nil
	}

	// no requeue needed, reconcile succeeded
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GithubIssueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&issuev1.GithubIssue{}).
		Complete(r)
}
