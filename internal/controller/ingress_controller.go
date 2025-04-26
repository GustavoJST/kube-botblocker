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
	"regexp"
	"strings"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var ingress networkingv1.Ingress
	if err := r.Get(ctx, req.NamespacedName, &ingress); err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "Ingress not found; ignoring since object must be deleted")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ann := ingress.GetAnnotations()
	if ann == nil {
		ann = make(map[string]string)
	}

	protected := ann[annotations.ProtectedIngressAnnotation] == "true"
	var changed bool

	if !protected {
		if current, ok := ann[annotations.IngressServerSnippet]; ok {
			log.Info("Started cleaning operation for Ingress")
			cleaned, err := updateAnnotation(current, "")
			if err != nil {
				log.Error(err, "failed cleaning Ingress snippet annotation")
				return ctrl.Result{}, nil
			}
			if cleaned == "" {
				delete(ann, annotations.IngressServerSnippet)
			} else {
				ann[annotations.IngressServerSnippet] = cleaned
			}
			changed = true
		}
		if _, ok := ann[annotations.IngressConfigNameAnnotation]; ok {
			delete(ann, annotations.IngressConfigNameAnnotation)
			changed = true
		}
	} else {
		cfgName, ok := ann[annotations.IngressConfigNameAnnotation]
		if !ok {
			log.Info(
				"Skipping protected Ingress without config name",
				"annotation", annotations.IngressConfigNameAnnotation,
			)
			return ctrl.Result{}, nil
		}

		var cfg v1alpha1.IngressConfig
		key := types.NamespacedName{Namespace: r.Environment.OperatorNamespace, Name: cfgName}
		if err := r.Get(ctx, key, &cfg); err != nil {
			if apierrors.IsNotFound(err) {
				log.Error(err, "IngressConfig not found", "ingressConfigName", cfgName)
			} else {
				log.Error(err, "Error fetching IngressConfig", "ingressConfigName", cfgName)
			}
			return ctrl.Result{}, nil
		}

		desired := buildNginxConfig(cfg.Spec.BlockedUserAgents)
		current := ann[annotations.IngressServerSnippet]
		updated, err := updateAnnotation(current, desired)
		if err != nil {
			log.Error(
				err,
				"Marker mismatch detect in current config; manual cleanup of all operator added configuration for the Ingress is required",
			)
			return ctrl.Result{}, nil
		}

		if updated != current {
			if updated == "" {
				delete(ann, annotations.IngressServerSnippet)
			} else {
				ann[annotations.IngressServerSnippet] = updated
			}
			changed = true
		}
	}

	if changed {
		ingress.SetAnnotations(ann)
		if err := r.Update(ctx, &ingress); err != nil {
			log.Error(err, "failed updating Ingress annotations")
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
		log.Info("Ingress annotations updated successfully")
	} else {
		log.Info("No changes detected; skipping update")
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

func updateAnnotation(currentConf, updatedConf string) (string, error) {
	startMarkerCount := strings.Count(currentConf, startMarker)
	endMarkerCount := strings.Count(currentConf, endMarker)

	if startMarkerCount != endMarkerCount || startMarkerCount > 1 && endMarkerCount > 1 {
		return "", fmt.Errorf(
			"mismatched or wrong number of start and end markers for kube-botblocker config. "+
				"Expected 1 start and end markers, got %d start and %d end markers",
			startMarkerCount, endMarkerCount,
		)
	}

	pattern := regexp.MustCompile("(?sm)^" + regexp.QuoteMeta(startMarker) + ".*?" + regexp.QuoteMeta(endMarker) + "$")

	// If updatedConf is empty, remove the entire block that matches the pattern
	if updatedConf == "" {
		return pattern.ReplaceAllLiteralString(currentConf, ""), nil
	}

	// Add updatedConf if currentConf is empty or doesn't have a valid kube-botblocker config
	// with start and end markers
	if !pattern.MatchString(currentConf) {
		return currentConf + fmt.Sprint("\n"+updatedConf), nil
	}

	return pattern.ReplaceAllLiteralString(currentConf, updatedConf), nil
}

var (
	startMarker = fmt.Sprintf("# %s operator: Configuration start\n", v1alpha1.GroupVersion.Group)
	endMarker   = fmt.Sprintf("# %s operator: Configuration end", v1alpha1.GroupVersion.Group)
)

func buildNginxConfig(userAgents []string) string {
	var sb strings.Builder
	pattern := strings.Join(userAgents, "|")
	sb.WriteString(startMarker)
	sb.WriteString("# Configuration added by kube-botblocker operator. Do not edit any of this manually\n")
	sb.WriteString(fmt.Sprintf(`if ($http_user_agent ~* "(%s)") {`, pattern))
	sb.WriteString("\n  return 403;\n")
	sb.WriteString("}\n")
	sb.WriteString(endMarker)
	return sb.String()
}

func (r *IngressReconciler) ReconcileFanOut(ctx context.Context, obj client.Object) []ctrl.Request {
	var (
		requests      = []ctrl.Request{}
		fanOutLog     = ctrl.Log.WithName("fanOutReconcile")
		ingressConfig = obj.(*v1alpha1.IngressConfig)
	)

	var ingressList networkingv1.IngressList
	if err := r.List(
		ctx,
		&ingressList,
		&client.MatchingFields{fmt.Sprintf("metadata.annotations.%s", annotations.ProtectedIngressAnnotation): "true"},
	); err != nil {
		fanOutLog.Error(err, "Failed to fetch list of protected ingresses")
		return requests
	}

	for _, ingress := range ingressList.Items {
		if ingress.Annotations[annotations.IngressConfigNameAnnotation] != ingressConfig.Name {
			continue
		}
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
