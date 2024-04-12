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
	"errors"
	"strings"

	githubStatusClient "github.com/jquad-group/github-status-controller/pkg/git"
	githubcontrollerpredicate "github.com/jquad-group/github-status-controller/pkg/predicate"
	tektondevv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PipelineRunReconciler reconciles a PipelineRun object
type PipelineRunReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const ANNOTATION_GITHUB_BASE_URL = "github-status-controller/github-base-url"
const ANNOTATION_GITHUB_OWNER = "github-status-controller/github-owner"
const ANNOTATION_GITHUB_REPOSITORY = "github-status-controller/github-repository"
const ANNOTATION_GITHUB_REVISION_PARAM_NAME = "github-status-controller/github-revision-param-name"
const ANNOTATION_GITHUB_SECRET_NAME = "github-status-controller/github-secret-name"
const ANNOTATION_GITHUB_SECRET_KEY = "github-status-controller/github-secret-key"

var labelSelector = v1.LabelSelector{
	MatchLabels: map[string]string{
		"github-status-controller": "enabled",
	},
}

var labelPredicate, _ = predicate.LabelSelectorPredicate(labelSelector)

//+kubebuilder:rbac:groups=tekton.dev.pipeline.jquad.rocks,resources=pipelineruns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tekton.dev.pipeline.jquad.rocks,resources=pipelineruns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tekton.dev.pipeline.jquad.rocks,resources=pipelineruns/finalizers,verbs=update

func (r *PipelineRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	var pipelineRun tektondevv1.PipelineRun
	if err := r.Get(ctx, req.NamespacedName, &pipelineRun); err != nil {
		return ctrl.Result{}, nil
	}

	if err := Validate(&pipelineRun); err != nil {
		return ctrl.Result{}, err
	}

	annotations := pipelineRun.GetAnnotations()
	baseUrl := annotations[ANNOTATION_GITHUB_BASE_URL]
	githubOwner := annotations[ANNOTATION_GITHUB_OWNER]
	githubRepository := annotations[ANNOTATION_GITHUB_REPOSITORY]
	githubRevisionParamName := annotations[ANNOTATION_GITHUB_REVISION_PARAM_NAME]
	githubSecretName := annotations[ANNOTATION_GITHUB_SECRET_NAME]
	githubSecretKey := annotations[ANNOTATION_GITHUB_SECRET_KEY]

	foundSecret := &core.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: githubSecretName, Namespace: pipelineRun.Namespace}, foundSecret); err != nil {
		return ctrl.Result{}, err
	}

	if err := ValidateSecret(&pipelineRun, *foundSecret); err != nil {
		return ctrl.Result{}, err
	}

	accessToken := string(foundSecret.Data[githubSecretKey])

	parts := strings.Split(githubRevisionParamName, ".")
	// format is "tasks.<task-name>.<param-name>"
	if len(parts) != 3 {
		return ctrl.Result{}, errors.New("the github revision param name doesn't have the correct format")
	}

	taskName := strings.Split(githubRevisionParamName, ".")[1]
	paramName := strings.Split(githubRevisionParamName, ".")[2]
	githubRevision, err := findGitHubRevision(pipelineRun.Status.PipelineSpec.Tasks, taskName, paramName)
	if err != nil {
		return ctrl.Result{}, err
	}

	insecureSkipVerify := false

	statusClient := githubStatusClient.NewGithubClient(baseUrl, githubOwner, githubRepository, githubRevision, accessToken, insecureSkipVerify)

	for _, c := range pipelineRun.Status.Conditions {
		if v1.ConditionStatus(c.Status) == v1.ConditionTrue {
			err, _ := statusClient.SetStatus("success", "The build is successful", "tekton-ci", "https://rancher.jquad.rocks")
			if err != nil {
				return ctrl.Result{}, err
			}
		} else if v1.ConditionStatus(c.Status) == v1.ConditionFalse {
			err, _ := statusClient.SetStatus("failure", "The build has failed", "tekton-ci", "https://rancher.jquad.rocks")
			if err != nil {
				return ctrl.Result{}, err
			}
		} else {
			err, _ := statusClient.SetStatus("pending", "The build is currently running", "tekton-ci", "https://rancher.jquad.rocks")
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	//fmt.Println("Name:" + pipelineRun.Name)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tektondevv1.PipelineRun{},
			builder.WithPredicates(
				githubcontrollerpredicate.StatusChangePredicate{},
				labelPredicate,
			)).
		Complete(r)
}

func Validate(pipelineRun *tektondevv1.PipelineRun) error {
	annotations := pipelineRun.GetAnnotations()
	_, existsBaseUrl := annotations[ANNOTATION_GITHUB_BASE_URL]
	if !existsBaseUrl {
		return errors.New("annotation '" + ANNOTATION_GITHUB_BASE_URL + "' is not set")
	}
	_, existsGithubOwner := annotations[ANNOTATION_GITHUB_OWNER]
	if !existsGithubOwner {
		return errors.New("annotation '" + ANNOTATION_GITHUB_OWNER + "' is not set")
	}
	_, existsGithubRepository := annotations[ANNOTATION_GITHUB_REPOSITORY]
	if !existsGithubRepository {
		return errors.New("annotation '" + ANNOTATION_GITHUB_REPOSITORY + "' is not set")
	}
	_, existsGithubRevisionParamName := annotations[ANNOTATION_GITHUB_REVISION_PARAM_NAME]
	if !existsGithubRevisionParamName {
		return errors.New("annotation '" + ANNOTATION_GITHUB_REVISION_PARAM_NAME + "' is not set")
	}

	_, existsGithubSecretName := annotations[ANNOTATION_GITHUB_SECRET_NAME]
	if !existsGithubSecretName {
		return errors.New("annotation '" + ANNOTATION_GITHUB_SECRET_NAME + "' is not set")
	}

	_, existsGithubSecretKey := annotations[ANNOTATION_GITHUB_SECRET_KEY]
	if !existsGithubSecretKey {
		return errors.New("annotation '" + ANNOTATION_GITHUB_SECRET_KEY + "' is not set")
	}

	return nil
}

func ValidateSecret(pipelineRun *tektondevv1.PipelineRun, secret core.Secret) error {

	annotations := pipelineRun.GetAnnotations()
	_, existsGithubSecretName := annotations[ANNOTATION_GITHUB_SECRET_NAME]

	_, existsGithubSecretKey := annotations[ANNOTATION_GITHUB_SECRET_KEY]

	if existsGithubSecretKey && existsGithubSecretName {
		if len(secret.Data[annotations[ANNOTATION_GITHUB_SECRET_KEY]]) <= 0 {
			return errors.New("'" + annotations[ANNOTATION_GITHUB_SECRET_KEY] + "' is not set in the secret '" + annotations[ANNOTATION_GITHUB_SECRET_NAME] + "' ")
		}
	}

	return nil
}

func findGitHubRevision(tasks []tektondevv1.PipelineTask, taskName, paramName string) (string, error) {
	for _, task := range tasks {
		if task.Name == taskName {
			for _, param := range task.Params {
				if param.Name == paramName {
					return param.Value.StringVal, nil
				}
			}
		}
	}
	return "", errors.New(paramName + " not found in task " + taskName) // Not found
}
