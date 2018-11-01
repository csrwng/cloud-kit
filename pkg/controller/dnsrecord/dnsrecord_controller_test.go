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

package dnsrecord

import (
	"context"
	"testing"

	log "github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/openshift/cloud-kit/pkg/apis"
	cloudkitv1 "github.com/openshift/cloud-kit/pkg/apis/cloudkit/v1alpha1"
	"github.com/openshift/cloud-kit/pkg/controller/util"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

const (
	testName      = "dnsrecord"
	testNamespace = "testns"
)

func TestDNSRecordReconcile(t *testing.T) {
	apis.AddToScheme(scheme.Scheme)

	// Utility function to get the test DNS record from the fake client
	getRecord := func(c client.Client) *cloudkitv1.DNSRecord {
		record := &cloudkitv1.DNSRecord{}
		err := c.Get(context.TODO(), client.ObjectKey{Name: testName, Namespace: testNamespace}, record)
		if err == nil {
			return record
		}
		return nil
	}

	tests := []struct {
		name          string
		existing      []runtime.Object
		actuator      Actuator
		expectErr     bool
		setupActuator (FakeActuator)
		validate      func(client.Client, FakeActuator)
	}{
		{
			name: "Add finalizer",
			existing: []runtime.Object{
				testDNSRecordWithoutFinalizer(),
			},
			validate: func(c client.Client, a Actuator) {
				r := getRecord(c)
				if r == nil || !util.HasFinalizer(r, cloudkitv1.DNSRecordFinalizer) {
					t.Errorf("did not get expected dnsrecord finalizer")
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := fake.NewFakeClient(test.existing...)
			actuator := testActuator()
			if test.setupActuator != nil {
				test.setupActuator(actuator)
			}
			rcd := &ReconcileDNSRecord{
				Client:   fakeClient,
				scheme:   scheme.Scheme,
				actuator: actuator,
				log:      log.WithField("controller", "dnsrecord-test"),
			}

			_, err := rcd.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      testName,
					Namespace: testNamespace,
				},
			})

			if test.validate != nil {
				test.validate(fakeClient, test.actuator)
			}

			if err != nil && !test.expectErr {
				t.Errorf("Unexpected error: %v", err)
			}
			if err == nil && test.expectErr {
				t.Errorf("Expected error but got none")
			}
		})
	}
}

func testDNSRecord() *cloudkitv1.DNSRecord {
	r := &cloudkitv1.DNSRecord{}
	r.Name = testName
	r.Namespace = testNamespace
	r.Finalizers = []string{cloudkitv1.DNSRecordFinalizer}
	r.UID = types.UID("1234")
	r.Spec = cloudkitv1.DNSRecordSpec{
		ZoneName:   "zone",
		RecordName: "record",
	}
	return r
}

func testDNSRecordWithoutFinalizer() *cloudkitv1.DNSRecord {
	r := testDNSRecord()
	r.Finalizers = []string{}
	return r
}

type fakeActuator struct {
	util.FakeActuator
}

func (a *fakeActuator) Create(r *cloudkitv1.DNSRecord) error {
	return a.FakeActuator.Create(r)
}
func (a *fakeActuator) Delete(r *cloudkitv1.DNSRecord) error {
	return a.FakeActuator.Delete(r)
}
func (a *fakeActuator) Update(r *cloudkitv1.DNSRecord) error {
	return a.FakeActuator.Update(r)
}
func (a *fakeActuator) Exists(r *cloudkitv1.DNSRecord) (bool, error) {
	return a.FakeActuator.Exists(r)
}

func testActuator() *fakeActuator {
	return &fakeActuator{
		FakeActuator: util.NewFakeActuator(),
	}
}
