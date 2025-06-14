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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/GustavoJST/kube-botblocker/api/v1alpha1"
	"github.com/GustavoJST/kube-botblocker/pkg/annotations"
)

// IngressConfigReconciler reconciles a IngressConfig object
type IngressConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kube-botblocker.github.io,resources=ingressconfigs,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=kube-botblocker.github.io,resources=ingressconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kube-botblocker.github.io,resources=ingressconfigs/finalizers,verbs=update

func (r *IngressConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var ingressConfig v1alpha1.IngressConfig
	if err := r.Get(ctx, req.NamespacedName, &ingressConfig); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if ingressConfig.Status.SpecHash == "" {
		specHash, err := hashObj(ingressConfig.Spec)
		if err != nil {
			log.Error(err, "Failed hashing ingressConfig Spec")
			return ctrl.Result{}, err
		}
		ingressConfig.Status.SpecHash = specHash
		return ctrl.Result{Requeue: true}, nil
	}

	// Check if lastUpdated is nil or Generation changed
	if ingressConfig.Status.LastUpdated == nil || ingressConfig.Generation != ingressConfig.Status.ObservedGeneration {
		now := metav1.NewTime(time.Now().UTC())
		specHash, err := hashObj(ingressConfig.Spec)
		if err != nil {
			log.Error(err, "Failed hashing IngressConfig Spec")
			return ctrl.Result{}, err
		}

		ingressConfig.Status.LastUpdated = &now
		ingressConfig.Status.ProtectedIngress.Total = 0
		ingressConfig.Status.ProtectedIngress.Updated = 0
		ingressConfig.Status.ObservedGeneration = ingressConfig.Generation
		ingressConfig.Status.SpecHash = specHash
		meta.SetStatusCondition(&ingressConfig.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.ConditionTypeUpdateSucceeded,
			Status:             metav1.ConditionFalse,
			Reason:             v1alpha1.ConditionReasonReconciliationInProgress,
			Message:            "Waiting for all Ingresses to be updated",
			LastTransitionTime: now,
		})

		// Update status so the ingress reconcile fanout can begin
		if err := r.Status().Update(ctx, &ingressConfig); err != nil {
			log.Error(err, "Failed to update IngresConfig status")
			return ctrl.Result{}, err
		}
		log.Info("Rolling update on all associated Ingresses")
		return ctrl.Result{}, nil
	}

	var ingressList networkingv1.IngressList
	if err := r.List(
		ctx,
		&ingressList,
		&client.MatchingFields{fmt.Sprintf("metadata.annotations.%s", annotations.IngressConfigNameAnnotation): ingressConfig.Name},
	); err != nil {
		return ctrl.Result{}, err
	}

	total := int32(len(ingressList.Items))
	updated := int32(0)
	for _, ing := range ingressList.Items {
		ann := ing.GetAnnotations()
		if ann[annotations.IngressConfigSpecHash] == ingressConfig.Status.SpecHash {
			updated++
		}
	}

	if ingressConfig.Status.ProtectedIngress.Total != total || ingressConfig.Status.ProtectedIngress.Updated != updated {
		ingressConfig.Status.ProtectedIngress.Total = total
		ingressConfig.Status.ProtectedIngress.Updated = updated

		if total == updated {
			meta.SetStatusCondition(&ingressConfig.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.ConditionTypeUpdateSucceeded,
				Status:             metav1.ConditionTrue,
				Reason:             v1alpha1.ConditionReasonReconciliationSuccessful,
				Message:            "All Ingresses successfully reconciled",
				LastTransitionTime: metav1.Now(),
			})

			if err := r.Status().Update(ctx, &ingressConfig); err != nil {
				log.Error(err, "Failed to update IngresConfig status")
				return ctrl.Result{}, err
			}
			log.Info("Finished updating associated Ingresses")
		}
	}

	if meta.IsStatusConditionPresentAndEqual(
		ingressConfig.Status.Conditions,
		v1alpha1.ConditionTypeUpdateSucceeded,
		metav1.ConditionTrue,
	) {
		return ctrl.Result{}, nil
	}

	// If there was an update but total != updated, then some Ingresses are still reconciling.
	// Restart the reconcile to check again until succeeded.
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *IngressConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.IngressConfig{}).
		Named("ingressconfig").
		Complete(r)
}

func hashObj(spec any) (string, error) {
	jsonBytes, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(jsonBytes)
	return hex.EncodeToString(hash[:]), nil
}
