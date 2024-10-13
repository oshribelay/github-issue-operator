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
	"github.com/go-logr/logr"
	"github.com/oshribelay/github-issue-operator/internal/controller/finalizer"
	"github.com/oshribelay/github-issue-operator/internal/controller/resources"
	"github.com/oshribelay/github-issue-operator/internal/controller/status"
	"github.com/oshribelay/github-issue-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"time"

	issuev1 "github.com/oshribelay/github-issue-operator/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GithubIssueReconciler reconciles a GithubIssue object
type GithubIssueReconciler struct {
	Client       client.Client
	GithubClient *resources.GithubClient
	Scheme       *runtime.Scheme
	Log          logr.Logger
}

// +kubebuilder:rbac:groups=issue.core.github.io,resources=githubissues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=issue.core.github.io,resources=githubissues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=issue.core.github.io,resources=githubissues/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *GithubIssueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("githubissue", req.NamespacedName)
	log.Info("Reconciling GithubIssue")

	// Fetch the GithubIssue custom resource
	githubIssue := &issuev1.GithubIssue{}
	if err := r.Client.Get(ctx, req.NamespacedName, githubIssue); err != nil {
		log.Info("Issue was deleted")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check if issue is marked for deletion (has DeletionTimestamp)
	if !githubIssue.GetDeletionTimestamp().IsZero() {
		// delete the issue from GitHub and remove it from the cluster
		if err := status.Delete(ctx, r.Client, r.GithubClient, githubIssue); err != nil {
			log.Error(err, "unable to delete GithubIssue")
			return ctrl.Result{}, err
		}

		if err := finalizer.RemoveFinalizer(ctx, r.Client, githubIssue); err != nil {
			log.Error(err, "unable to remove finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Fetch the associated Secret to get the token
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("%s-token-secret", githubIssue.Name),
		Namespace: githubIssue.Namespace,
	}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			// Secret not found, create it
			err = resources.CreateSecret(githubIssue, r.Client, ctx)
			if err != nil {
				return ctrl.Result{}, err
			}
			// Update status to indicate token is required
			if err := status.UpdateTokenRequired(ctx, r.Client, githubIssue, true); err != nil {
				log.Error(err, "unable to update TokenRequired status")
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch token from secret
	token, exists := secret.Data["token"]
	if !exists || len(token) == 0 {
		log.Info("GitHub token missing in secret, requeueing...")
		// Update status to indicate token is required
		if err := status.UpdateTokenRequired(ctx, r.Client, githubIssue, true); err != nil {
			log.Error(err, "unable to update TokenRequired status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Token exists, update status to indicate token is not required
	if err := status.UpdateTokenRequired(ctx, r.Client, githubIssue, false); err != nil {
		log.Error(err, "unable to update TokenRequired status")
		return ctrl.Result{}, err
	}
	// initialize GitHub Client dynamically with the token from the secret
	r.GithubClient = resources.NewGithubClient(string(token))

	if err := finalizer.EnsureFinalizer(ctx, r.Client, githubIssue); err != nil {
		log.Error(err, "unable to add finalizer")
		return ctrl.Result{Requeue: true}, err // If there's an error ensuring the finalizer, requeue
	}

	owner, repo, err := utils.ParseRepoUrl(githubIssue.Spec.Repo)
	if err != nil {
		log.Error(err, "unable to parse repo url")
		return ctrl.Result{}, err
	}
	title := githubIssue.Spec.Title
	description := githubIssue.Spec.Description
	issueNumber := githubIssue.Status.IssueNumber

	// no issue number in status check existence by title
	issue, err := r.GithubClient.CheckIssueExists(owner, repo, title, int(issueNumber))
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
		updatedIssue, err := r.GithubClient.UpdateIssue(owner, repo, issue, description, title)
		if err != nil {
			log.Error(err, "unable to update issue")
			return ctrl.Result{}, err
		}
		issue = updatedIssue
	}
	// update the status of the GithubIssue CR
	if err := status.Update(ctx, r.Client, githubIssue, issue); err != nil {
		if apierrors.IsConflict(err) {
			log.Info("conflict occurred, requeueing...")
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
		log.Error(err, "unable to update GithubIssue")
		return ctrl.Result{}, err
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
