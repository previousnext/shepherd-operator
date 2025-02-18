package restore

import (
	"context"
	"testing"

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

	instance := &extensionv1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: corev1.NamespaceDefault,
		},
		Spec: extensionv1.RestoreSpec{},
	}

	// Query which will be used to find our Restore object.
	query := types.NamespacedName{
		Name:      instance.ObjectMeta.Name,
		Namespace: instance.ObjectMeta.Namespace,
	}

	rd := &ReconcileRestore{
		Client: fake.NewFakeClient(instance),
		scheme: scheme.Scheme,
	}

	_, err := rd.Reconcile(reconcile.Request{
		NamespacedName: query,
	})
	assert.Nil(t, err)

	found := &extensionv1.Restore{}
	err = rd.Client.Get(context.TODO(), query, found)
	assert.Nil(t, err)
}
