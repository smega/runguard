// patch.go — computes the mutation applied to an opted-in CronJob so its
// main container's command gets wrapped with `runguard wrap`, without the
// user having to edit their own manifest by hand.
//
// What gets added to the CronJob's pod template:
//   - an initContainer ("runguard-init") that downloads the static runguard
//     binary into a shared emptyDir volume (the same approach already
//     documented as a manual fallback in docs/KUBERNETES.md, just automated)
//   - that volume, mounted into both the init container and the first
//     container of the pod
//   - an env var RUNGUARD_KEY on the first container, sourced from a
//     per-CronJob Secret (not inlined as plaintext in the CronJob spec)
//   - the first container's Command/Args rewritten to run
//     "/runguard-bin/runguard wrap --key=$(RUNGUARD_KEY) -- <original command>"
//
// Deliberate limitation: this only works if the container already sets an
// explicit Command (the common, recommended pattern for CronJobs, and the
// one shown in docs/KUBERNETES.md). If a container relies entirely on its
// image's own ENTRYPOINT/CMD with no Command/Args set in the manifest, the
// Kubernetes API gives no way to know what that is, so injection is skipped
// — the job is still registered with RunGuard (so the user gets a key), but
// nothing is auto-wrapped, and the CronJob is annotated to say why.
package main

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	annotationMonitor          = "runguard.io/monitor"
	annotationJobName          = "runguard.io/job-name"
	annotationSeverity         = "runguard.io/severity"
	annotationInjected         = "runguard.io/injected"
	annotationInjectionSkipped = "runguard.io/injection-skipped-reason"
	annotationRegisteredJobID  = "runguard.io/job-id"
	volumeName                 = "runguard-bin"
	volumeMountPath            = "/runguard-bin"
	initContainerName          = "runguard-init"
	binaryInstallURLDefault    = "https://runguard.smega.eu/install.sh"
	secretKeyField             = "key"
)

// wantsMonitoring reports whether a CronJob has opted in via annotation.
func wantsMonitoring(cj *batchv1.CronJob) bool {
	return cj.Annotations[annotationMonitor] == "true"
}

// alreadyInjected reports whether this CronJob was already patched by a
// previous reconcile — avoids re-wrapping an already-wrapped command, and
// avoids re-registering (and re-patching) on every poll once settled.
func alreadyInjected(cj *batchv1.CronJob) bool {
	return cj.Annotations[annotationInjected] == "true" || cj.Annotations[annotationInjectionSkipped] != ""
}

// monitorName returns the name to register with RunGuard for this CronJob:
// an explicit runguard.io/job-name annotation if set, else a cluster-scoped
// default that stays unique across namespaces and (if --cluster-name was
// given) across clusters sharing one RunGuard account.
func monitorName(clusterName string, cj *batchv1.CronJob) string {
	if v := cj.Annotations[annotationJobName]; v != "" {
		return v
	}
	// Separated with "." rather than "/": the backend's register handler
	// only strips spaces when turning a job name into a heartbeat key
	// (strings.ReplaceAll(name, " ", "_")), so a literal "/" here would
	// end up inside the key itself and break the /hb/:key route (the
	// router stops matching :key at the next "/", producing a silent
	// 404 on every heartbeat -- found by actually testing against a
	// live cluster, not by inspection).
	if clusterName != "" {
		return fmt.Sprintf("%s.%s.%s", clusterName, cj.Namespace, cj.Name)
	}
	return fmt.Sprintf("%s.%s", cj.Namespace, cj.Name)
}

// secretName is the per-CronJob Secret this operator owns, holding the
// heartbeat key. Namespaced alongside the CronJob itself.
func secretName(cj *batchv1.CronJob) string {
	return cj.Name + "-runguard-key"
}

// buildKeySecret returns the Secret to create/update for this CronJob.
func buildKeySecret(cj *batchv1.CronJob, heartbeatKey string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metaWithOwner(secretName(cj), cj.Namespace, cj),
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{secretKeyField: heartbeatKey},
	}
}

// injectionResult describes what happened when trying to wrap a CronJob's
// container, for logging/annotating purposes.
type injectionResult struct {
	skippedReason string // non-empty if injection couldn't be applied
}

// applyInjection mutates cj in place: adds the shared volume, the
// install-binary init container, and rewrites the first container's
// command/env/volumeMounts to route through `runguard wrap`. Returns a
// non-empty skippedReason if the container has no explicit Command to wrap.
func applyInjection(cj *batchv1.CronJob, installURL string) injectionResult {
	podSpec := &cj.Spec.JobTemplate.Spec.Template.Spec
	if len(podSpec.Containers) == 0 {
		return injectionResult{skippedReason: "CronJob has no containers"}
	}
	target := &podSpec.Containers[0]

	if len(target.Command) == 0 {
		// No explicit Command set — the image's own ENTRYPOINT/CMD is opaque
		// to the Kubernetes API, so there's nothing safe to prepend to.
		return injectionResult{
			skippedReason: fmt.Sprintf(
				"container %q has no explicit command set; add one (see docs/KUBERNETES.md) so runguard knows what to wrap",
				target.Name,
			),
		}
	}

	if url := installURL; url == "" {
		url = binaryInstallURLDefault
	} else {
		installURL = url
	}
	if installURL == "" {
		installURL = binaryInstallURLDefault
	}

	ensureVolume(podSpec)
	ensureInitContainer(podSpec, installURL)

	target.VolumeMounts = append(target.VolumeMounts, corev1.VolumeMount{
		Name:      volumeName,
		MountPath: volumeMountPath,
	})
	target.Env = append(target.Env, corev1.EnvVar{
		Name: "RUNGUARD_KEY",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName(cj)},
				Key:                  secretKeyField,
			},
		},
	})

	original := append(append([]string{}, target.Command...), target.Args...)
	target.Command = []string{volumeMountPath + "/runguard"}
	target.Args = append([]string{"wrap", "--key=$(RUNGUARD_KEY)", "--"}, original...)

	return injectionResult{}
}

func ensureVolume(podSpec *corev1.PodSpec) {
	for _, v := range podSpec.Volumes {
		if v.Name == volumeName {
			return
		}
	}
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name:         volumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	})
}

func ensureInitContainer(podSpec *corev1.PodSpec, installURL string) {
	for _, ic := range podSpec.InitContainers {
		if ic.Name == initContainerName {
			return
		}
	}
	podSpec.InitContainers = append(podSpec.InitContainers, corev1.Container{
		Name:  initContainerName,
		Image: "curlimages/curl:8.11.0",
		Command: []string{
			"/bin/sh", "-c",
			fmt.Sprintf(
				"curl -fsSL %s -o %s/runguard && chmod +x %s/runguard",
				installURL, volumeMountPath, volumeMountPath,
			),
		},
		VolumeMounts: []corev1.VolumeMount{{Name: volumeName, MountPath: volumeMountPath}},
	})
}
