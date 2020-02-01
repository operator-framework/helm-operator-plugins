package controllerutil

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
