/*
Copyright 2023.

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

package handler

import (
	"context"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crthandler "sigs.k8s.io/controller-runtime/pkg/handler"
)

type requeueFilter struct {
	crthandler.EventHandler
}

// RequeueFilter wraps the EventHandler and skips the event if it was requeued for the given object
func RequeueFilter(delegate crthandler.EventHandler) crthandler.EventHandler {
	return &requeueFilter{EventHandler: delegate}
}

func (r *requeueFilter) Create(ctx context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	r.EventHandler.Create(ctx, evt, &delegatingQueue{RateLimitingInterface: q})
}

func (r *requeueFilter) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	r.EventHandler.Update(ctx, evt, &delegatingQueue{RateLimitingInterface: q})
}

func (r *requeueFilter) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	r.EventHandler.Delete(ctx, evt, &delegatingQueue{RateLimitingInterface: q})
}

func (r *requeueFilter) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	r.EventHandler.Generic(ctx, evt, &delegatingQueue{RateLimitingInterface: q})
}

type delegatingQueue struct {
	workqueue.RateLimitingInterface
}

func (q *delegatingQueue) Add(item interface{}) {
	if q.RateLimitingInterface.NumRequeues(item) > 0 {
		return
	}
	q.RateLimitingInterface.Add(item)
}
