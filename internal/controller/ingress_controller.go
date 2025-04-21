/*
Copyright 2025.

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
	"strings"
	"time"
	"regexp"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/GustavoJST/kube-botblocker/api/v1alpha1"
	"github.com/GustavoJST/kube-botblocker/pkg/annotations"
	"github.com/GustavoJST/kube-botblocker/pkg/environment"
)

// IngressReconciler reconciles a Ingress object
type IngressReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	Environment *environment.OperatorEnv
}

// +kubebuilder:rbac:groups=kube-botblocker.github.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kube-botblocker.github.io,resources=ingresses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kube-botblocker.github.io,resources=ingresses/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Ingress object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var ingress networkingv1.Ingress
	if err := r.Get(ctx, req.NamespacedName, &ingress); err != nil {
		if errors.IsNotFound(err) {
			log.Error(err, "Reconciled Ingress resource was not found")
		}
		return ctrl.Result{}, err
	}

	ingressConfigName, ok := ingress.GetAnnotations()[annotations.IngressConfigNameAnnotation]
	if !ok {
		log.Info(fmt.Sprintf("Ingress is missing annotation '%s'. Skipping reconciliation", annotations.IngressConfigNameAnnotation))
		return ctrl.Result{}, nil
	}

	if _, ok := ingress.GetAnnotations()[annotations.ProtectedIngressAnnotation]; !ok {
		currentConfig, exist := ingress.GetAnnotations()[annotations.IngressServerSnippet]
		if exist {
			// Removes config if found
			updateAnnotation(currentConfig, "")
			log.Info("Cleaned up existing config from ingress")
		}

		delete(ingress.Annotations, annotations.IngressConfigNameAnnotation)
		if err := r.Update(ctx, &ingress); err != nil {
			log.Error(err, "Error updating ingress on clean up")
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
		return ctrl.Result{}, nil
	}

	var ingressConfig v1alpha1.IngressConfig
	ingressConfigNamespacedName := types.NamespacedName{
		Namespace: r.Environment.OperatorNamespace,
		Name:      ingressConfigName,
	}
	if err := r.Get(ctx, ingressConfigNamespacedName, &ingressConfig); err != nil {
		if errors.IsNotFound(err) {
			log.Error(
				err, "IngressConfig resource associated with this ingress was not found",
				"ingressConfigName", ingressConfigName,
				"ingressConfigNamespace", r.Environment.OperatorNamespace,
			)
		}
		return ctrl.Result{}, nil
	}


	nginxConfig := buildNginxConfig(ingressConfig.Spec.BlockedUserAgents)
	currentConfig := ingress.GetAnnotations()[annotations.IngressServerSnippet]
	updatedConfig := updateAnnotation(currentConfig, nginxConfig)


	// TODO: Test for reconcile looping
	if currentConfig == updatedConfig {
		log.Info("Ingress configuration up to date")
		return ctrl.Result{}, nil
	} else {
		// TODO: for debug purposes only
		log.Info("NOT EQUAL HERE")
	}

	ingress.Annotations[annotations.IngressServerSnippet] = updatedConfig
	log.Info("Updating configuration in Ingress resource")
	if err := r.Update(ctx, &ingress); err != nil {
		log.Error(err, fmt.Sprintf("Failed to update ingress with '%s' annotation", annotations.IngressServerSnippet))
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&networkingv1.Ingress{},
		fmt.Sprintf("metadata.annotations.%s", annotations.ProtectedIngressAnnotation),
		func(rawObj client.Object) []string {
			ingress := rawObj.(*networkingv1.Ingress)
			protectionEnabled := ingress.GetAnnotations()[annotations.ProtectedIngressAnnotation]

			if protectionEnabled == "" {
				return nil
			}

			return []string{protectionEnabled}
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}, builder.WithPredicates(ingressPredicate())).
		Watches(&v1alpha1.IngressConfig{}, handler.EnqueueRequestsFromMapFunc(r.ReconcileFanOut)).
		Named("ingress").
		Complete(r)
}

func updateAnnotation(currentConf, updatedConf string) string {
	pattern := regexp.MustCompile("(?s)" + regexp.QuoteMeta(startMarker) + ".*?" + regexp.QuoteMeta(endMarker))

	// If updatedConf is empty, remove the entire block
	if updatedConf == "" {
	return pattern.ReplaceAllString(currentConf, "")

	}
	// Add updatedConf if currentConf is empty or doesn't have a valid kube-botblocker config
	// with start and end markers
	if !pattern.MatchString(currentConf) {
		return currentConf + updatedConf
	}

	return pattern.ReplaceAllString(currentConf, updatedConf)
}

func (r *IngressReconciler) ReconcileFanOut(ctx context.Context, obj client.Object) []ctrl.Request {
	var (
		requests  = []ctrl.Request{}
		fanOutLog = ctrl.Log.WithName("fanOutReconcile")
	)

	var ingressList networkingv1.IngressList
	if err := r.List(
		ctx,
		&ingressList,
		&client.MatchingFields{fmt.Sprintf("metadata.annotations.%s", annotations.ProtectedIngressAnnotation): "true"},
	); err != nil {
		fanOutLog.Error(err, "Failed to fetch list of protected ingresses")
	}

	for _, ingress := range ingressList.Items {
		request := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: ingress.GetNamespace(),
				Name:      ingress.GetName(),
			},
		}
		requests = append(requests, request)
	}

	return requests
}

func ingressPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return checkIngressAnnotations(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			result := false
			if e.ObjectOld.GetAnnotations()[annotations.ProtectedIngressAnnotation] == "true" || e.ObjectNew.GetAnnotations()[annotations.ProtectedIngressAnnotation] == "true" {
				result = true
			}
			return result
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return checkIngressAnnotations(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return checkIngressAnnotations(e.Object)
		},
	}
}

func checkIngressAnnotations(obj client.Object) bool {
	objAnnotations := obj.(*networkingv1.Ingress).GetAnnotations()

	result := false
	if objAnnotations[annotations.ProtectedIngressAnnotation] == "true" && objAnnotations[annotations.IngressConfigNameAnnotation] != "" {
		result = true
	}
	return result
}

var (
	startMarker = fmt.Sprintf("\n# %s operator: Configuration start\n", v1alpha1.GroupVersion.Group)
	// Este newline no final pode trazer problemas se não tiver um espaço vazio no final da annotation. Talvez remover
	endMarker   = fmt.Sprintf("# %s operator: Configuration end\n", v1alpha1.GroupVersion.Group)
)

func buildNginxConfig(userAgents []string) string {
	var sb strings.Builder
	pattern := strings.Join(userAgents, "|")

	sb.WriteString(startMarker)
	sb.WriteString("# Configuration added by kube-botblocker operator. Do not edit any of this manually\n")
	sb.WriteString(fmt.Sprintf(`if ($http_user_agent ~* "(%s)") {`, pattern))
	sb.WriteString("\nreturn 403\n")
	sb.WriteString("}\n")
	sb.WriteString(endMarker)
	return sb.String()
}
