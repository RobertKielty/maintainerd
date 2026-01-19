/*
Copyright 2026.

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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	maintainerdcncfiov1alpha1 "github.com/cncf/maintainer-d/code-scanners/api/v1alpha1"
)

// CodeScannerSnykReconciler reconciles a CodeScannerSnyk object
type CodeScannerSnykReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=maintainer-d.cncf.io,resources=codescannersnyks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=maintainer-d.cncf.io,resources=codescannersnyks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=maintainer-d.cncf.io,resources=codescannersnyks/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *CodeScannerSnykReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the CodeScannerSnyk instance
	snyk := &maintainerdcncfiov1alpha1.CodeScannerSnyk{}
	if err := r.Get(ctx, req.NamespacedName, snyk); err != nil {
		if errors.IsNotFound(err) {
			log.Info("CodeScannerSnyk resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get CodeScannerSnyk")
		return ctrl.Result{}, err
	}

	// 2. Build the ConfigMap
	configMap := r.configMapForSnyk(snyk)

	// 3. Set owner reference for garbage collection
	if err := ctrl.SetControllerReference(snyk, configMap, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference on ConfigMap")
		return ctrl.Result{}, err
	}

	// 4. Create or Update the ConfigMap
	existingCM := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKeyFromObject(configMap), existingCM)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating ConfigMap", "name", configMap.Name, "namespace", configMap.Namespace)
		if err := r.Create(ctx, configMap); err != nil {
			log.Error(err, "Failed to create ConfigMap")
			return ctrl.Result{}, err
		}
	} else if err != nil {
		log.Error(err, "Failed to get ConfigMap")
		return ctrl.Result{}, err
	} else {
		// Update existing ConfigMap
		existingCM.Data = configMap.Data
		if err := r.Update(ctx, existingCM); err != nil {
			log.Error(err, "Failed to update ConfigMap")
			return ctrl.Result{}, err
		}
	}

	// 5. Add lineage annotation to CR
	configMapRef := fmt.Sprintf("%s/%s", configMap.Namespace, configMap.Name)
	if snyk.Annotations == nil {
		snyk.Annotations = make(map[string]string)
	}
	if snyk.Annotations[AnnotationConfigMapRef] != configMapRef {
		snyk.Annotations[AnnotationConfigMapRef] = configMapRef
		if err := r.Update(ctx, snyk); err != nil {
			log.Error(err, "Failed to update CodeScannerSnyk annotation")
			return ctrl.Result{}, err
		}
	}

	// 6. Update status
	snyk.Status.ConfigMapRef = configMapRef
	if err := r.Status().Update(ctx, snyk); err != nil {
		log.Error(err, "Failed to update CodeScannerSnyk status")
		return ctrl.Result{}, err
	}

	log.Info("Reconciliation complete", "configMap", configMapRef)
	return ctrl.Result{}, nil
}

func (r *CodeScannerSnykReconciler) configMapForSnyk(snyk *maintainerdcncfiov1alpha1.CodeScannerSnyk) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snyk.Name,
			Namespace: snyk.Namespace,
		},
		Data: map[string]string{
			ConfigMapKeyCodeScanner: ScannerTypeSnyk,
			ConfigMapKeyProjectName: snyk.Spec.ProjectName,
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *CodeScannerSnykReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&maintainerdcncfiov1alpha1.CodeScannerSnyk{}).
		Owns(&corev1.ConfigMap{}).
		Named("codescannersnyk").
		Complete(r)
}
