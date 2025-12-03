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
	"time"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1alpha1 "github.com/emzm17/expose2/api/v1alpha1"
)

// MicroserviceReconciler reconciles a Microservice object
type MicroserviceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=api.example.com,resources=microservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=api.example.com,resources=microservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=api.example.com,resources=microservices/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Microservice object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *MicroserviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	ms := &apiv1alpha1.Microservice{}
	err := r.Get(ctx, req.NamespacedName, ms)
	if err != nil {
		return ctrl.Result{}, err
	}

	// create a deployment.......
	deployment := v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ms.Name,
			Namespace: ms.Namespace,
		},
		Spec: v1.DeploymentSpec{
			Replicas: &ms.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": ms.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": ms.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  ms.Name,
							Image: ms.Spec.Image,
							Ports: []corev1.ContainerPort{
								{ContainerPort: ms.Spec.Port},
							},
						},
					},
				},
			},
		},
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(ms, &deployment, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Create or Update the Deployment
	existingDeploy := &v1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: ms.Name, Namespace: ms.Namespace}, existingDeploy)
	if errors.IsNotFound(err) {
		//log.Info("Creating Deployment", "name", ms.Name)
		if err := r.Create(ctx, &deployment); err != nil {
			return ctrl.Result{}, err
		}
	} else if err == nil {
		deployment.ResourceVersion = existingDeploy.ResourceVersion
		//log.Info("Updating Deployment", "name", ms.Name)
		if err := r.Update(ctx, &deployment); err != nil {
			return ctrl.Result{}, err
		}
	}

	//----------- Create Service -----------------
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ms.Name,
			Namespace: ms.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": ms.Name},
			Ports: []corev1.ServicePort{
				{
					Port:       ms.Spec.Port,
					TargetPort: intstr.FromInt(int(ms.Spec.TargetPort)),
				},
			},
		},
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(ms, &svc, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	var existingSvc corev1.Service
	err = r.Get(ctx, types.NamespacedName{Name: ms.Name, Namespace: ms.Namespace}, &existingSvc)
	if errors.IsNotFound(err) {
		// log.Info("Creating Service", "name", ms.Name)
		if err := r.Create(ctx, &svc); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update status
	updatedDeploy := &v1.Deployment{}
	_ = r.Get(ctx, types.NamespacedName{Name: ms.Name, Namespace: ms.Namespace}, updatedDeploy)

	ms.Status.ReadyReplicas = updatedDeploy.Status.ReadyReplicas
	_ = r.Status().Update(ctx, ms)

	// Requeue after 1 minute to check again
	return ctrl.Result{RequeueAfter: time.Minute}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *MicroserviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.Microservice{}).
		Complete(r)
}
