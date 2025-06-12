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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

var (
	HasIngressConfigNameAnnotation = "HasIngressConfigNameAnnotation"
)

// +kubebuilder:rbac:groups=kube-botblocker.github.io,resources=ingressconfigs,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;patch;update;watch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	var ingress networkingv1.Ingress
	if err := r.Get(ctx, req.NamespacedName, &ingress); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Ingress not found")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Erro fetching Ingress")
		return ctrl.Result{}, err
	}

	ann := ingress.GetAnnotations()
	if ann == nil {
		ann = make(map[string]string)
	}
	ingressConfigName, protected := ann[annotations.IngressConfigNameAnnotation]
	changed := false

	if protected {
		var ingressConfig v1alpha1.IngressConfig
		key := types.NamespacedName{Namespace: r.Environment.OperatorNamespace, Name: ingressConfigName}
		if err := r.Get(ctx, key, &ingressConfig); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Specified IngressConfig not found in operator namespace; skipping update", "ingressConfigName", ingressConfigName)
				return ctrl.Result{}, nil
			}
			log.Error(err, "Error fetching IngressConfig", "ingressConfigName", ingressConfigName)
			return ctrl.Result{}, err
		}

		desiredSnippet := buildNginxConfig(ingressConfig.Spec.BlockedUserAgents)
		currentSnippet := ann[annotations.IngressServerSnippet]

		updatedSnippet, err := updateServerSnippet(currentSnippet, desiredSnippet)
		if err != nil {
			log.Error(err, "Failed to create updated server-snippet annotation configuration")
		}

		if updatedSnippet != currentSnippet {
			ann[annotations.IngressServerSnippet] = updatedSnippet
			changed = true
		}
	} else {
		// Ingress had the ingressConfigName annotation removed. Previously added configuration
		// by the operator should be cleaned up
		if currentSnippet, ok := ann[annotations.IngressServerSnippet]; ok {
			log.Info("Started cleaning operation for Ingress")
			cleaned, err := updateServerSnippet(currentSnippet, "")
			if err != nil {
				log.Error(err, "Failed cleaning Ingress server-snippet annotation")
				return ctrl.Result{}, err
			}
			if cleaned == "" {
				delete(ann, annotations.IngressServerSnippet)
			} else {
				ann[annotations.IngressServerSnippet] = cleaned
			}
		}
		delete(ann, annotations.IngressConfigNameAnnotation)
		delete(ann, annotations.LastUpdatedAnnotation)
		changed = true
	}

	if changed {
		ann[annotations.LastUpdatedAnnotation] = metav1.Now().Add(2 * time.Second).UTC().Format(time.RFC3339)
		ingress.SetAnnotations(ann)
		if err := r.Update(ctx, &ingress); err != nil {
			log.Error(err, "Failed updating Ingress annotations")
			return ctrl.Result{}, err
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
		fmt.Sprintf("metadata.annotations.%s", annotations.IngressConfigNameAnnotation),
		func(rawObj client.Object) []string {
			ingress := rawObj.(*networkingv1.Ingress)
			ingressConfigName := ingress.GetAnnotations()[annotations.IngressConfigNameAnnotation]
			if ingressConfigName == "" {
				return nil
			}
			return []string{ingressConfigName}
		},
	); err != nil {
		return err
	}

	// Indexer for checking if IngressConfigName Annotation exists
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&networkingv1.Ingress{},
		HasIngressConfigNameAnnotation,
		func(rawObj client.Object) []string {
			ingress := rawObj.(*networkingv1.Ingress)
			if ingress.GetAnnotations()[annotations.IngressConfigNameAnnotation] != "" {
				return []string{"true"}
			}
			return nil
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}, builder.WithPredicates(ingressPredicate())).
		Watches(
			&v1alpha1.IngressConfig{},
			handler.EnqueueRequestsFromMapFunc(r.ReconcileFanOut),
			builder.WithPredicates(ingressConfigPredicate()),
		).
		Named("ingress").
		Complete(r)
}

var (
	startMarker = fmt.Sprintf("# %s operator: Configuration start\n", v1alpha1.GroupVersion.Group)
	endMarker   = fmt.Sprintf("# %s operator: Configuration end", v1alpha1.GroupVersion.Group)
)

func updateServerSnippet(currentConf, updatedConf string) (string, error) {
	startMarkerCount := strings.Count(currentConf, startMarker)
	endMarkerCount := strings.Count(currentConf, endMarker)

	if startMarkerCount != endMarkerCount || startMarkerCount > 1 && endMarkerCount > 1 {
		return "", fmt.Errorf(
			"mismatched or wrong number of start and end markers for kube-botblocker config. "+
				"Expected 1 start and end markers, got %d start and %d end markers. Manual action required",
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
		&client.MatchingFields{HasIngressConfigNameAnnotation: "true"},
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

func ingressConfigPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			objNew := e.ObjectNew.(*v1alpha1.IngressConfig)
			return meta.IsStatusConditionPresentAndEqual(
				objNew.Status.Conditions,
				v1alpha1.ConditionTypeUpdateSucceeded,
				metav1.ConditionFalse,
			)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return meta.IsStatusConditionPresentAndEqual(
				e.Object.(*v1alpha1.IngressConfig).Status.Conditions,
				v1alpha1.ConditionTypeUpdateSucceeded,
				metav1.ConditionFalse,
			)
		},
	}
}

func ingressPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.(*networkingv1.Ingress).GetAnnotations()[annotations.IngressConfigNameAnnotation] != ""
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			result := false
			annOld := e.ObjectOld.GetAnnotations()[annotations.IngressConfigNameAnnotation]
			annNew := e.ObjectNew.GetAnnotations()[annotations.IngressConfigNameAnnotation]
			if annOld != "" || annNew != "" {
				result = true
			}
			return result
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
}

// func checkIngressAnnotations(obj client.Object) bool {
// 	ingAnnotations := obj.(*networkingv1.Ingress).GetAnnotations()
// 	return ingAnnotations[annotations.IngressConfigNameAnnotation] != ""
// }
