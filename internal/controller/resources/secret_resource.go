package resources

import (
	"context"
	"fmt"
	issuev1 "github.com/oshribelay/github-issue-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateSecret(githubIssue *issuev1.GithubIssue, c client.Client, ctx context.Context) error {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-token-secret", githubIssue.Name),
			Namespace: githubIssue.Namespace,

			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(githubIssue, schema.GroupVersionKind{
					Group:   "issue.core.github.io",
					Version: "v1",
					Kind:    "GithubIssue",
				}),
			},
		},
		StringData: map[string]string{
			"token": "",
		},
	}

	err := c.Create(ctx, &secret)
	if err != nil {
		return err
	}
	return nil
}
