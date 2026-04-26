/*
Copyright 2026.
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
package controller

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// FailableResource is implemented by any resource that has a settable
// failure reason and observed generation — allowing shared error handling
// logic across different controller types.
type FailableResource interface {
	client.Object
	SetFailedStatus(reason string)
	SetObservedGeneration(generation int64)
}

// setFailedStatus is a generic helper that re-fetches the latest version
// of any FailableResource and marks it as failed with a reason.
// Re-fetching before update avoids resource version conflict errors.
func setFailedStatus(ctx context.Context, c client.Client, obj FailableResource, reason string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Re-fetch the latest version to avoid conflict errors
	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		log.Error(err, "Failed to re-fetch resource before setting failed status")
		return ctrl.Result{}, err
	}

	obj.SetFailedStatus(reason)
	obj.SetObservedGeneration(obj.GetGeneration())

	if err := c.Status().Update(ctx, obj); err != nil {
		log.Error(err, "Failed to update failure status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
