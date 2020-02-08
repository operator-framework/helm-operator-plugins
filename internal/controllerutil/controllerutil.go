package controllerutil

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	AddFinalizer      = controllerutil.AddFinalizer
	RemoveFinalizer   = controllerutil.RemoveFinalizer
	ContainsFinalizer = func(obj metav1.Object, finalizer string) bool {
		for _, f := range obj.GetFinalizers() {
			if f == finalizer {
				return true
			}
		}
		return false
	}
)

func WaitForDeletion(ctx context.Context, cl client.Client, o runtime.Object) error {
	key, err := client.ObjectKeyFromObject(o)
	if err != nil {
		return err
	}

	return wait.PollImmediateUntil(time.Millisecond*10, func() (bool, error) {
		err := cl.Get(ctx, key, o)
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	}, ctx.Done())
}
