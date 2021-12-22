/*
Copyright 2020 The Operator-SDK Authors.

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

package fake

import (
	"context"

	"github.com/go-logr/logr"
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
func (c *Controller) Start(ctx context.Context) error {
	c.Started = true
	<-ctx.Done()
	return ctx.Err()
}
func (c *Controller) Reconcile(_ context.Context, r reconcile.Request) (reconcile.Result, error) {
	c.ReconcileRequests = append(c.ReconcileRequests, r)
	return reconcile.Result{}, nil
}

func (c Controller) GetLogger() logr.Logger {
	return logr.Discard()
}
