/*
Copyright 2022.

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

package controllers

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/arshabbir/testop/api/v1alpha1"
	databasev1alpha1 "github.com/arshabbir/testop/api/v1alpha1"
)

// DbReconciler reconciles a Db object
type DbReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=database.dev.db.com,resources=dbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.dev.db.com,resources=dbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database.dev.db.com,resources=dbs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Db object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *DbReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	// TODO(user): your logic here

	db := v1alpha1.Db{}

	r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, &db)

	l.Info("Reconsile Invoked ...", req.Name, req.Namespace, db.Spec.Name, db.Spec.Image)

	//  TODO : Need to add mysql pod creation logic
	if db.Spec.Name != db.Status.Name {
		db.Status.Name = db.Spec.Name
		l.Info("Sentting status for CRD to", db.Spec.Name, db.Status.Name)
		r.Status().Update(ctx, &db)
	}
	//dbpod := corev1.Pod{}

	// Check if the deployment already exists, if not create a new deployment.
	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: db.Name, Namespace: db.Namespace}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			// Define and create a new deployment.
			dep := r.deploymentForMysql(&db)
			if err = r.Create(ctx, dep); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		} else {
			return ctrl.Result{}, err
		}
	}

	// Ensure the deployment size is the same as the spec.
	var size int32 = int32(db.Spec.Replica)
	if *found.Spec.Replicas != int32(size) {
		found.Spec.Replicas = &size
		if err = r.Update(ctx, found); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Update the Db status with the pod names.
	// List the pods for this CR's deployment.
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(db.Namespace),
		client.MatchingLabels(labelsForApp(db.Name)),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		return ctrl.Result{}, err
	}

	// Update status.Nodes if needed.
	// podNames := getPodNames(podList.Items)
	// if !reflect.DeepEqual(podNames, db.Status.Replica) {
	// 	memcached.Status.Nodes = podNames
	// 	if err := r.Status().Update(ctx, &db); err != nil {
	// 		return ctrl.Result{}, err
	// 	}
	// }

	// create mysql service

	service := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: db.Name, Namespace: db.Namespace}, service)
	if err != nil {
		if errors.IsNotFound(err) {
			l.Info("Creating MySql Service........")
			// Define and create a new deployment.
			service := r.serviceForMysql(&db)
			if err = r.Create(ctx, service); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		} else {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *DbReconciler) serviceForMysql(db *v1alpha1.Db) *corev1.Service {
	lbls := labelsForApp(db.Name)
	s := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      db.Name,
			Namespace: db.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: "NodePort",
			Ports: []corev1.ServicePort{
				{
					Name: "mysqlport",
					Port: 3306,
				},
			},
			Selector: lbls,
		},
	}
	return s

}

func (r *DbReconciler) deploymentForMysql(m *v1alpha1.Db) *appsv1.Deployment {
	lbls := labelsForApp(m.Name)
	var replicas int32 = int32(m.Spec.Replica)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: lbls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: lbls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: m.Spec.Image,
						Name:  m.Spec.Name,
						Ports: []corev1.ContainerPort{{
							ContainerPort: 3306,
							Name:          "mysqlport",
						}},
						Env: []corev1.EnvVar{{
							Name:  "MYSQL_USER",
							Value: "mysql",
						},
							{
								Name:  "MYSQL_PASSWORD",
								Value: "password",
							},
							{
								Name:  "MYSQL_DATABASE",
								Value: "movidb",
							},
							{
								Name:  "MYSQL_ROOT_PASSWORD",
								Value: "password",
							},
						},
					}},
				},
			},
		},
	}

	// Set Mysql instance as the owner and controller.memcac
	// NOTE: calling SetControllerReference, and setting owner references in
	// general, is important as it allows deleted objects to be garbage collected.
	controllerutil.SetControllerReference(m, dep, r.Scheme)
	return dep
}

// labelsForApp creates a simple set of labels for Memcached.
func labelsForApp(name string) map[string]string {
	return map[string]string{"cr_name": name}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DbReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.Db{}).
		Owns(&v1.Deployment{}).
		Complete(r)
}
