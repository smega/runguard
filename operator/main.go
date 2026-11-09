// runguard-operator: watches Kubernetes CronJobs across a cluster and
// auto-registers + auto-wraps the ones opted in via annotation, so RunGuard
// monitoring needs zero manual `runguard wrap` editing for the common case
// (a CronJob with an explicit container Command).
//
// This is a deliberately simple poll-based reconciler, not a full
// controller-runtime operator with watch+informer caching — a CronJob
// fleet is small and changes rarely, so polling every --poll-interval is
// more than fast enough and a lot less code to get right. See
// docs/KUBERNETES.md in the main repo for the manual alternative this
// automates, and patch.go for exactly what gets mutated on each CronJob.
//
// Opt in per CronJob with:
//
//	metadata:
//	  annotations:
//	    runguard.io/monitor: "true"
//
// Optional annotations: runguard.io/job-name (override the registered job
// name), runguard.io/severity (low|normal|high|critical).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const version = "0.1.0"

func main() {
	var (
		apiBase      = flag.String("api", envOr("RUNGUARD_API", ""), "RunGuard API base URL (or RUNGUARD_API env var) — required")
		agentToken   = flag.String("agent-token", os.Getenv("RUNGUARD_AGENT_TOKEN"), "RunGuard agent token (or RUNGUARD_AGENT_TOKEN env var)")
		clusterName  = flag.String("cluster-name", os.Getenv("RUNGUARD_CLUSTER_NAME"), "Optional label prefixed onto registered job names, to disambiguate CronJobs with the same namespace/name across multiple clusters on one RunGuard account")
		namespace    = flag.String("namespace", "", "Restrict to one namespace (default: all namespaces — requires the ClusterRole in deploy/)")
		pollInterval = flag.Duration("poll-interval", 30*time.Second, "How often to re-scan for CronJobs")
		installURL   = flag.String("install-url", envOr("RUNGUARD_INSTALL_URL", ""), "URL the init container downloads the runguard binary from (or RUNGUARD_INSTALL_URL env var) — required for auto-wrapping; without it, CronJobs still get registered but are annotated as skipped")
		kubeconfig   = flag.String("kubeconfig", "", "Path to kubeconfig (default: in-cluster config; falls back to ~/.kube/config for local dev)")
	)
	flag.Parse()

	if *apiBase == "" {
		log.Fatal("--api or RUNGUARD_API is required (the base URL of your RunGuard instance)")
	}
	if *agentToken == "" {
		log.Fatal("--agent-token or RUNGUARD_AGENT_TOKEN is required (find it on the account page)")
	}

	clientset, err := buildKubeClient(*kubeconfig)
	if err != nil {
		log.Fatalf("build kubernetes client: %v", err)
	}
	rg := newRunguardClient(*apiBase, *agentToken)

	log.Printf("runguard-operator %s starting — api=%s namespace=%q cluster-name=%q poll-interval=%s",
		version, *apiBase, orAll(*namespace), *clusterName, *pollInterval)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(*pollInterval)
	defer ticker.Stop()

	// Run once immediately, then on every tick, so you don't wait a full
	// poll-interval after startup to see the first result.
	reconcileAll(ctx, clientset, rg, *namespace, *clusterName, *installURL)
	for {
		select {
		case <-ctx.Done():
			log.Println("shutting down")
			return
		case <-ticker.C:
			reconcileAll(ctx, clientset, rg, *namespace, *clusterName, *installURL)
		}
	}
}

func reconcileAll(ctx context.Context, clientset *kubernetes.Clientset, rg *runguardClient, namespace, clusterName, installURL string) {
	cronJobs, err := clientset.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("list cronjobs: %v", err)
		return
	}

	var scanned, eligible, acted int
	for i := range cronJobs.Items {
		cj := &cronJobs.Items[i]
		scanned++
		if !wantsMonitoring(cj) {
			continue
		}
		eligible++
		if alreadyInjected(cj) {
			continue
		}
		acted++
		if err := reconcileOne(ctx, clientset, rg, cj, clusterName, installURL); err != nil {
			log.Printf("reconcile %s/%s: %v", cj.Namespace, cj.Name, err)
		}
	}
	log.Printf("reconcile pass: scanned=%d opted-in=%d acted-on=%d", scanned, eligible, acted)
}

// reconcileOne registers cj with RunGuard, creates its heartbeat-key
// Secret, applies the command-wrapping patch, and persists both — logging
// and annotating a skip reason instead of failing hard if the container
// has no explicit Command to wrap (see patch.go).
func reconcileOne(ctx context.Context, clientset *kubernetes.Clientset, rg *runguardClient, cj *batchv1.CronJob, clusterName, installURL string) error {
	name := monitorName(clusterName, cj)
	severity := cj.Annotations[annotationSeverity]

	resp, err := rg.register(ctx, name, cj.Spec.Schedule, severity)
	if err != nil {
		return fmt.Errorf("register %q: %w", name, err)
	}
	log.Printf("registered %q (job_id=%s, created=%v) for %s/%s", name, resp.JobID, resp.Created, cj.Namespace, cj.Name)

	if err := upsertSecret(ctx, clientset, buildKeySecret(cj, resp.Key)); err != nil {
		return fmt.Errorf("upsert secret: %w", err)
	}

	result := applyInjection(cj, installURL)
	if cj.Annotations == nil {
		cj.Annotations = map[string]string{}
	}
	cj.Annotations[annotationRegisteredJobID] = resp.JobID
	if result.skippedReason != "" {
		cj.Annotations[annotationInjectionSkipped] = result.skippedReason
		log.Printf("%s/%s: registered but not auto-wrapped: %s", cj.Namespace, cj.Name, result.skippedReason)
	} else {
		cj.Annotations[annotationInjected] = "true"
	}

	if _, err := clientset.BatchV1().CronJobs(cj.Namespace).Update(ctx, cj, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update cronjob: %w", err)
	}
	return nil
}

func upsertSecret(ctx context.Context, clientset *kubernetes.Clientset, secret *corev1.Secret) error {
	_, err := clientset.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		_, err = clientset.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
	}
	return err
}

func buildKubeClient(kubeconfigPath string) (*kubernetes.Clientset, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		path := kubeconfigPath
		if path == "" {
			if home, herr := os.UserHomeDir(); herr == nil {
				path = filepath.Join(home, ".kube", "config")
			}
		}
		cfg, err = clientcmd.BuildConfigFromFlags("", path)
		if err != nil {
			return nil, fmt.Errorf("no in-cluster config and no usable kubeconfig at %q: %w", path, err)
		}
		log.Printf("using out-of-cluster kubeconfig: %s", path)
	}
	return kubernetes.NewForConfig(cfg)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func orAll(ns string) string {
	if ns == "" {
		return "<all>"
	}
	return ns
}
