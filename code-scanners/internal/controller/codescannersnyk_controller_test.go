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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	maintainerdcncfiov1alpha1 "github.com/cncf/maintainer-d/code-scanners/api/v1alpha1"
)

func TestCodeScannerSnykReconciler_Reconcile(t *testing.T) {
	const resourceName = "test-snyk"
	const namespace = "default"

	typeNamespacedName := types.NamespacedName{
		Name:      resourceName,
		Namespace: namespace,
	}

	// Create the custom resource
	resource := &maintainerdcncfiov1alpha1.CodeScannerSnyk{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: namespace,
		},
	}

	if err := k8sClient.Create(context.Background(), resource); err != nil {
		t.Fatalf("Failed to create CodeScannerSnyk: %v", err)
	}

	defer func() {
		if err := k8sClient.Delete(context.Background(), resource); err != nil {
			t.Logf("Failed to delete CodeScannerSnyk: %v", err)
		}
	}()

	// Test reconciliation
	controllerReconciler := &CodeScannerSnykReconciler{
		Client: k8sClient,
		Scheme: k8sClient.Scheme(),
	}

	_, err := controllerReconciler.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: typeNamespacedName,
	})
	if err != nil {
		t.Errorf("Reconcile() error = %v", err)
	}
}
