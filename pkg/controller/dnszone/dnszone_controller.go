/*
Copyright 2018 The Kubernetes Authors.

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

package dnszone

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cloudkitv1 "github.com/openshift/cloud-kit/pkg/apis/cloudkit/v1alpha1"
	"github.com/openshift/cloud-kit/pkg/controller/util"
)

// AddWithActuator creates a new DNSZone Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func AddWithActuator(mgr manager.Manager, actuator Actuator) error {
	return add(mgr, newReconciler(mgr, actuator))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return &ReconcileDNSZone{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		actuator: actuator,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("dnszone-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to DNSZone
	err = c.Watch(&source.Kind{Type: &cloudkitv1.DNSZone{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileDNSZone{}

// ReconcileDNSZone reconciles a DNSZone object
type ReconcileDNSZone struct {
	client.Client
	scheme   *runtime.Scheme
	actuator Actuator
}

// Reconcile reads that state of the cluster for a DNSZone object and makes changes based on the state read
// and what is in the DNSZone.Spec
// Automatically generate RBAC rules to allow the Controller to read and write DNSZones
// +kubebuilder:rbac:groups=cloudkit.openshift.io,resources=dnszones,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileDNSZone) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the DNSZone instance
	zone := &cloudkitv1.DNSZone{}
	err := r.Get(context.TODO(), request.NamespacedName, zone)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// If DNS zone hasn't been deleted and doesn't have a finalizer, add one
	if zone.ObjectMeta.DeletionTimestamp.IsZero() &&
		!util.HasFinalizer(zone, cloudkitv1.DNSZoneFinalizer) {
		util.AddFinalizer(zone, cloudkitv1.DNSZoneFinalizer)
		if err = r.Update(context.Background(), zone); err != nil {
			return reconcile.Result{}, err
		}
	}

	if !zone.ObjectMeta.DeletionTimestamp.IsZero() {
		// no-op if finalizer has been removed.
		if !util.HasFinalizer(zone, cloudkitv1.DNSZoneFinalizer) {
			// glog.Infof("reconciling machine object %v causes a no-op as there is no finalizer.", name)
			return reconcile.Result{}, nil
		}
		// glog.Infof("reconciling machine object %v triggers delete.", name)
		if err := r.actuator.Delete(zone); err != nil {
			// glog.Errorf("Error deleting machine object %v; %v", name, err)
			return reconcile.Result{}, err
		}

		// Remove finalizer on successful deletion.
		// glog.Infof("machine object %v deletion successful, removing finalizer.", name)
		util.DeleteFinalizer(zone, cloudkitv1.DNSZoneFinalizer)
		if err := r.Client.Update(context.Background(), zone); err != nil {
			// glog.Errorf("Error removing finalizer from machine object %v; %v", name, err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	exist, err := r.actuator.Exists(zone)
	if err != nil {
		// glog.Errorf("Error checking existence of machine instance for machine object %v; %v", name, err)
		return reconcile.Result{}, err
	}
	if exist {
		// glog.Infof("Reconciling machine object %v triggers idempotent update.", name)
		err := r.actuator.Update(zone)
		if err != nil {
			// glog.Warningf("unable to update machine %v: %v", name, err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}
	// glog.Infof("Reconciling machine object %v triggers idempotent create.", m.ObjectMeta.Name)
	if err := r.actuator.Create(zone); err != nil {
		// glog.Warningf("unable to create machine %v: %v", name, err)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}
