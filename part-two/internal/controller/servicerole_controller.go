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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	iamv1alpha1 "github.com/craft-global-psk/psk-iam-operator/api/v1alpha1"
)

// ServiceRoleReconciler reconciles a ServiceRole object
type ServiceRoleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=iam.craft-conf.com,resources=serviceroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=iam.craft-conf.com,resources=serviceroles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=iam.craft-conf.com,resources=serviceroles/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ServiceRole object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *ServiceRoleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the ServiceRole
	var serviceRole iamv1alpha1.ServiceRole
	if err := r.Get(ctx, req.NamespacedName, &serviceRole); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get ServiceRole")
		return ctrl.Result{}, err
	}

	// Create or update the ServiceAccount
	serviceAccountName := fmt.Sprintf("%s-sa", serviceRole.Name)
	logger.Info(serviceAccountName)
	if err := r.ensureServiceAccount(ctx, &serviceRole, serviceAccountName); err != nil {
		logger.Error(err, "Failed to ensure ServiceAccount")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ServiceRoleReconciler) ensureServiceAccount(ctx context.Context, serviceRole *iamv1alpha1.ServiceRole, name string) error {
	logger := log.FromContext(ctx)

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: serviceRole.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "iam-operator",
				"platform.io/servicerole":      serviceRole.Name,
			},
		},
	}

	// Set owner reference for automatic cleanup
	if err := controllerutil.SetControllerReference(serviceRole, serviceAccount, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	// Check if ServiceAccount exists
	existing := &corev1.ServiceAccount{}
	err := r.Get(ctx, client.ObjectKey{Name: name, Namespace: serviceRole.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create new ServiceAccount
			if err := r.Create(ctx, serviceAccount); err != nil {
				return fmt.Errorf("failed to create ServiceAccount: %w", err)
			}
			logger.Info("Created ServiceAccount", "name", name)
		} else {
			return fmt.Errorf("failed to get ServiceAccount: %w", err)
		}
	} else {
		// Update existing ServiceAccount if needed
		if existing.Labels == nil {
			existing.Labels = make(map[string]string)
		}

		updated := false
		for key, value := range serviceAccount.Labels {
			if existing.Labels[key] != value {
				existing.Labels[key] = value
				updated = true
			}
		}

		if updated {
			if err := r.Update(ctx, existing); err != nil {
				return fmt.Errorf("failed to update ServiceAccount: %w", err)
			}
			logger.Info("Updated ServiceAccount", "name", name)
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceRoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iamv1alpha1.ServiceRole{}).
		Complete(r)
}
