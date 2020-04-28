package fake

import (
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type Controller struct {
	WatchCalls        []watchCall
	Started           bool
	ReconcileRequests []reconcile.Request
}

type watchCall struct {
	Source     source.Source
	Handler    handler.EventHandler
	Predicates []predicate.Predicate
}

var _ controller.Controller = &Controller{}

func (c *Controller) Watch(s source.Source, h handler.EventHandler, ps ...predicate.Predicate) error {
	c.WatchCalls = append(c.WatchCalls, watchCall{s, h, ps})
	return nil
}
func (c *Controller) Start(stop <-chan struct{}) error {
	c.Started = true
	<-stop
	return nil
}
func (c *Controller) Reconcile(r reconcile.Request) (reconcile.Result, error) {
	c.ReconcileRequests = append(c.ReconcileRequests, r)
	return reconcile.Result{}, nil
}
