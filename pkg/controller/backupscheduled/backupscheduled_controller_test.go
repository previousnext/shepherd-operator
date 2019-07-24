/*
Copyright 2019 University of Adelaide.

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

package backupscheduled

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/universityofadelaide/shepherd-operator/pkg/apis"
	extensionv1 "github.com/universityofadelaide/shepherd-operator/pkg/apis/extension/v1"
)

func TestReconcile(t *testing.T) {
	apis.AddToScheme(scheme.Scheme)

	instance := &extensionv1.BackupScheduled{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: corev1.NamespaceDefault,
			Labels: map[string]string{
				"site": "foo",
			},
		},
		Spec: extensionv1.BackupScheduledSpec{
			Schedule: "0 0 * * * *",
		},
	}

	// Query which will be used to find our BackupScheduled object.
	query := types.NamespacedName{
		Name:      instance.ObjectMeta.Name,
		Namespace: instance.ObjectMeta.Namespace,
	}

	rd := &ReconcileBackupScheduled{
		Client: fake.NewFakeClient(instance),
		scheme: scheme.Scheme,
	}

	_, err := rd.Reconcile(reconcile.Request{
		NamespacedName: query,
	})
	assert.Nil(t, err)

	found := &extensionv1.BackupScheduled{}
	err = rd.Client.Get(context.TODO(), query, found)
	assert.Nil(t, err)
}

func TestReconcileNoLabels(t *testing.T) {
	apis.AddToScheme(scheme.Scheme)

	instance := &extensionv1.BackupScheduled{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: corev1.NamespaceDefault,
		},
	}

	// Query which will be used to find our BackupScheduled object.
	query := types.NamespacedName{
		Name:      instance.ObjectMeta.Name,
		Namespace: instance.ObjectMeta.Namespace,
	}

	rd := &ReconcileBackupScheduled{
		Client: fake.NewFakeClient(instance),
		scheme: scheme.Scheme,
	}

	_, err := rd.Reconcile(reconcile.Request{
		NamespacedName: query,
	})
	assert.Error(t, err, "BackupScheduled doesn't have a site label.")
}

func TestReconcileNoSchedule(t *testing.T) {
	apis.AddToScheme(scheme.Scheme)

	instance := &extensionv1.BackupScheduled{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: corev1.NamespaceDefault,
			Labels: map[string]string{
				"site": "foo",
			},
		},
	}

	// Query which will be used to find our BackupScheduled object.
	query := types.NamespacedName{
		Name:      instance.ObjectMeta.Name,
		Namespace: instance.ObjectMeta.Namespace,
	}

	rd := &ReconcileBackupScheduled{
		Client: fake.NewFakeClient(instance),
		scheme: scheme.Scheme,
	}

	_, err := rd.Reconcile(reconcile.Request{
		NamespacedName: query,
	})
	assert.Error(t, err, "BackupScheduled doesn't have a schedule.")
}

func TestReconcileInvalidSchedule(t *testing.T) {
	apis.AddToScheme(scheme.Scheme)

	instance := &extensionv1.BackupScheduled{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: corev1.NamespaceDefault,
			Labels: map[string]string{
				"site": "foo",
			},
		},
		Spec: extensionv1.BackupScheduledSpec{
			Schedule: "a b * * * * *",
		},
	}

	// Query which will be used to find our BackupScheduled object.
	query := types.NamespacedName{
		Name:      instance.ObjectMeta.Name,
		Namespace: instance.ObjectMeta.Namespace,
	}

	rd := &ReconcileBackupScheduled{
		Client: fake.NewFakeClient(instance),
		scheme: scheme.Scheme,
	}

	_, err := rd.Reconcile(reconcile.Request{
		NamespacedName: query,
	})
	assert.Contains(t, err.Error(), "syntax error in ")
}

func TestGetScheduleComparison(t *testing.T) {
	spec1 := extensionv1.BackupScheduledStatus{}
	now1 := time.Now()
	assert.Equal(t, now1, getScheduleComparison(spec1, now1), "comparison time defaults to now when nil value")

	d, _ := time.Parse(time.RFC3339, time.RFC3339)
	spec2 := extensionv1.BackupScheduledStatus{
		LastExecutedTime: &metav1.Time{Time: d},
	}
	now2 := time.Now()
	assert.Equal(t, d, getScheduleComparison(spec2, now2), "comparison time defaults to last executed time")
}