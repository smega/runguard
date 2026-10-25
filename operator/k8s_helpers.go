package main

import (
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// metaWithOwner builds ObjectMeta for a resource owned by the given
// CronJob, so Kubernetes garbage-collects it automatically when the
// CronJob is deleted (the operator never needs its own delete-on-cleanup
// logic for these).
func metaWithOwner(name, namespace string, cj *batchv1.CronJob) metav1.ObjectMeta {
	truePtr := true
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels: map[string]string{
			"app.kubernetes.io/managed-by": "runguard-operator",
		},
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion:         "batch/v1",
			Kind:               "CronJob",
			Name:               cj.Name,
			UID:                cj.UID,
			Controller:         &truePtr,
			BlockOwnerDeletion: &truePtr,
		}},
	}
}
