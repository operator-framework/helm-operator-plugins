package reconcilerutil

import (
	"fmt"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type Object interface {
	runtime.Object
	metav1.Object
}

type ActionConfigGetter interface {
	ActionConfigFor(obj Object) (*action.Configuration, error)
}

func NewActionConfigGetter(cfg *rest.Config, rm meta.RESTMapper, log logr.Logger) ActionConfigGetter {
	return &actionConfigGetter{
		cfg:        cfg,
		restMapper: rm,
		log:        log,
	}
}

var _ ActionConfigGetter = &actionConfigGetter{}

type actionConfigGetter struct {
	cfg        *rest.Config
	restMapper meta.RESTMapper
	log        logr.Logger
}

func (acg *actionConfigGetter) ActionConfigFor(obj Object) (*action.Configuration, error) {
	// Create a RESTClientGetter
	rcg, err := newRESTClientGetter(obj.GetNamespace(), acg.cfg, acg.restMapper)
	if err != nil {
		return nil, err
	}

	// Setup the debug log function that Helm will use
	debugLog := func(format string, v ...interface{}) {
		acg.log.V(1).Info(fmt.Sprintf(format, v...))
	}

	// Create a client that helm will use to manage release resources.
	// The passed object is used as an owner reference on every
	// object the client creates.
	kc := kube.New(rcg)
	ownerRef := metav1.NewControllerRef(obj, obj.GetObjectKind().GroupVersionKind())
	oric := newOwnerRefInjectingClient(*kc, *ownerRef)

	// Create the Kubernetes Secrets client. The passed object is
	// also used as an owner reference in the release secrets
	// created by this client.
	kcs, err := cmdutil.NewFactory(rcg).KubernetesClientSet()
	if err != nil {
		return nil, err
	}
	d := driver.NewSecrets(&ownerRefSecretClient{
		SecretInterface: kcs.CoreV1().Secrets(obj.GetNamespace()),
		refs:            []metav1.OwnerReference{*ownerRef},
	})

	// Also, use the debug log for the storage driver
	d.Log = debugLog

	// Initialize the storage backend
	s := storage.Init(d)

	return &action.Configuration{
		RESTClientGetter: rcg,
		Releases:         s,
		KubeClient:       oric,
		Log:              debugLog,
	}, nil
}
