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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// reconcileTotal counts every reconcile loop by controller and outcome.
	reconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "oci_operator_reconcile_total",
			Help: "Total number of reconciliations by controller and result.",
		},
		[]string{"controller", "result"}, // result: success | error
	)

	// ociAPICallDuration measures latency for each OCI API operation.
	ociAPICallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "oci_operator_oci_api_call_duration_seconds",
			Help:    "Duration of OCI API calls in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"controller", "operation", "result"}, // result: success | error
	)

	// phaseTransitions counts how many times each lifecycle phase is entered.
	phaseTransitions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "oci_operator_phase_transitions_total",
			Help: "Total number of resource phase transitions by controller and target phase.",
		},
		[]string{"controller", "phase"},
	)
)

const (
	metricResultSuccess = "success"
	metricResultError   = "error"
)

func init() {
	metrics.Registry.MustRegister(reconcileTotal, ociAPICallDuration, phaseTransitions)
}

// measureOCICall runs fn, records its duration and result in ociAPICallDuration, and returns the error.
func measureOCICall(controller, operation string, fn func() error) error {
	start := time.Now()
	err := fn()
	result := metricResultSuccess
	if err != nil {
		result = metricResultError
	}
	ociAPICallDuration.WithLabelValues(controller, operation, result).Observe(time.Since(start).Seconds())
	return err
}
