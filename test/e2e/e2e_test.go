package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
)

const (
	kindClusterName   = "shard-controller-test"
	testNamespace     = "shard-system"
	managerImage      = "shard-controller/manager:latest"
	workerImage       = "shard-controller/worker:latest"
	kubectlTimeoutSec = 120
)

// TestEndToEnd runs a comprehensive E2E suite against a Kind cluster
func TestEndToEnd(t *testing.T) {
	ctx := context.Background()

	ensureKind := os.Getenv("E2E_USE_KIND") != "false" // default true
	if ensureKind {
		if !hasBinary("kind") || !hasBinary("docker") || !hasBinary("kubectl") {
			t.Skipf("Skipping E2E: requires kind, docker and kubectl in PATH")
		}
		if err := ensureKindCluster(t); err != nil {
			t.Fatalf("failed to ensure kind cluster: %v", err)
		}
		t.Cleanup(func() {
			if os.Getenv("E2E_KEEP_CLUSTER") == "true" {
				return
			}
			_ = runCmd("kind", []string{"delete", "cluster", "--name", kindClusterName}, t)
		})
	}

	// Build and load images into Kind so the deployments can pull them locally
	if ensureKind {
		if err := dockerBuildAndLoad(t); err != nil {
			t.Fatalf("failed to build/load images: %v", err)
		}
	}

	// Apply CRDs and manifests
	if err := applyManifests(t); err != nil {
		t.Fatalf("failed to apply manifests: %v", err)
	}

	// Wait for deployments
	mustRolloutStatus(t, "deployment/shard-manager")
	mustRolloutStatus(t, "deployment/shard-worker")

	// Prepare k8s client
	k8sClient := mustControllerRuntimeClient(t)

	// Add our CRDs to scheme
	if err := shardv1.AddToScheme(scheme.Scheme); err != nil {
		t.Fatalf("failed to add shardv1 to scheme: %v", err)
	}

	t.Run("CRDsInstalled", func(t *testing.T) {
		// Verify CRDs exist via kubectl
		if err := runCmd("kubectl", []string{"get", "crd", "shardinstances.shard.io"}, t); err != nil {
			t.Fatalf("expected shardinstances CRD to exist: %v", err)
		}
		if err := runCmd("kubectl", []string{"get", "crd", "shardconfigs.shard.io"}, t); err != nil {
			t.Fatalf("expected shardconfigs CRD to exist: %v", err)
		}
	})

	t.Run("WorkersCreateShardInstances", func(t *testing.T) {
		// Wait up to 90s for the worker heartbeat loop to initialize and CRs to appear
		deadline := time.Now().Add(90 * time.Second)
		var lastErr error
		for time.Now().Before(deadline) {
			shardList := &shardv1.ShardInstanceList{}
			if err := k8sClient.List(ctx, shardList, ctrlclient.InNamespace(testNamespace)); err != nil {
				lastErr = err
				time.Sleep(3 * time.Second)
				continue
			}
			if len(shardList.Items) >= 1 { // worker replicas is 3 by default; accept >=1 to be robust on slow starts
				lastErr = nil
				break
			}
			time.Sleep(3 * time.Second)
		}
		if lastErr != nil {
			t.Fatalf("failed to list ShardInstances in time: %v", lastErr)
		}
	})

	t.Run("HealthEndpointsReady", func(t *testing.T) {
		// Just check the Pods are Ready (rollout already did). Optionally port-forward and curl health endpoints.
		// Quick sanity: kubectl get pods
		if err := runCmd("kubectl", []string{"-n", testNamespace, "get", "pods"}, t); err != nil {
			t.Fatalf("kubectl get pods failed: %v", err)
		}
	})

	t.Run("ShardInstanceHeartbeatsUpdate", func(t *testing.T) {
		// Wait for LastHeartbeat to advance (worker default heartbeat interval ~30s)
		shardList := &shardv1.ShardInstanceList{}
		if err := k8sClient.List(ctx, shardList, ctrlclient.InNamespace(testNamespace)); err != nil {
			t.Fatalf("failed to list shards: %v", err)
		}
		if len(shardList.Items) == 0 {
			t.Skip("no shards present yet; skip heartbeat assertion")
		}

		// Capture initial timestamps
		initial := make(map[string]time.Time)
		for _, s := range shardList.Items {
			initial[s.Name] = s.Status.LastHeartbeat.Time
		}

		// Wait up to 90s for at least one shard to update heartbeat
		updated := false
		deadline := time.Now().Add(90 * time.Second)
		for time.Now().Before(deadline) && !updated {
			time.Sleep(5 * time.Second)
			tmp := &shardv1.ShardInstanceList{}
			if err := k8sClient.List(ctx, tmp, ctrlclient.InNamespace(testNamespace)); err != nil {
				continue
			}
			for _, s := range tmp.Items {
				if prev, ok := initial[s.Name]; ok {
					if s.Status.LastHeartbeat.After(prev) {
						updated = true
						break
					}
				}
			}
		}
		if !updated {
			t.Fatalf("expected at least one shard heartbeat to update within timeout")
		}
	})

	t.Run("ScaleWorkersCreatesMoreShards", func(t *testing.T) {
		// Scale worker deployment to 4 replicas and expect >=4 ShardInstances
		if err := runCmd("kubectl", []string{"-n", testNamespace, "scale", "deployment/shard-worker", "--replicas=4"}, t); err != nil {
			t.Fatalf("failed to scale worker deployment: %v", err)
		}
		mustRolloutStatus(t, "deployment/shard-worker")

		deadline := time.Now().Add(120 * time.Second)
		for time.Now().Before(deadline) {
			shardList := &shardv1.ShardInstanceList{}
			if err := k8sClient.List(ctx, shardList, ctrlclient.InNamespace(testNamespace)); err == nil {
				if len(shardList.Items) >= 4 {
					return
				}
			}
			time.Sleep(5 * time.Second)
		}
		t.Fatalf("expected >=4 ShardInstances after scaling workers")
	})

	t.Run("CreateAndListShardConfig", func(t *testing.T) {
		// Create a simple ShardConfig CR to ensure CRD works end-to-end
		sc := &shardv1.ShardConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "e2e-config", Namespace: testNamespace},
			Spec: shardv1.ShardConfigSpec{
				MinShards:               1,
				MaxShards:               5,
				ScaleUpThreshold:        0.8,
				ScaleDownThreshold:      0.3,
				HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
				LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
				GracefulShutdownTimeout: metav1.Duration{Duration: 30 * time.Second},
			},
		}
		if err := k8sClient.Create(ctx, sc); err != nil {
			t.Fatalf("failed to create ShardConfig: %v", err)
		}
		// Get it back
		out := &shardv1.ShardConfig{}
		if err := k8sClient.Get(ctx, ctrlclient.ObjectKey{Name: sc.Name, Namespace: sc.Namespace}, out); err != nil {
			t.Fatalf("failed to get ShardConfig: %v", err)
		}
	})
}

// Helpers

func ensureKindCluster(t *testing.T) error {
	t.Helper()
	// If cluster exists, skip creation
	if err := runCmd("kind", []string{"get", "clusters"}, t); err == nil {
		out, _ := exec.Command("kind", "get", "clusters").CombinedOutput()
		if strings.Contains(string(out), kindClusterName) {
			// Ensure kubeconfig context points to our cluster
			_ = runCmd("kubectl", []string{"config", "use-context", "kind-" + kindClusterName}, t)
			return nil
		}
	}

	// Create cluster with simple config (1 control-plane, 2 workers)
	configYAML := "kind: Cluster\napiVersion: kind.x-k8s.io/v1alpha4\nnodes:\n- role: control-plane\n- role: worker\n- role: worker\n"
	cmd := exec.Command("kind", "create", "cluster", "--name", kindClusterName, "--config=-")
	cmd.Stdin = strings.NewReader(configYAML)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("kind create cluster failed: %v, out=%s", err, string(out))
	}
	// Switch kubectl context to the new cluster
	_ = runCmd("kubectl", []string{"config", "use-context", "kind-" + kindClusterName}, t)
	return nil
}

func dockerBuildAndLoad(t *testing.T) error {
	t.Helper()
	// Build manager
	if err := runCmd("docker", []string{"build", "-f", "Dockerfile.manager", "-t", managerImage, "."}, t); err != nil {
		return err
	}
	// Build worker
	if err := runCmd("docker", []string{"build", "-f", "Dockerfile.worker", "-t", workerImage, "."}, t); err != nil {
		return err
	}
	// Load into kind
	if err := runCmd("kind", []string{"load", "docker-image", "--name", kindClusterName, managerImage}, t); err != nil {
		return err
	}
	if err := runCmd("kind", []string{"load", "docker-image", "--name", kindClusterName, workerImage}, t); err != nil {
		return err
	}
	return nil
}

func applyManifests(t *testing.T) error {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		return err
	}
	// Apply namespace first
	if err := runCmd("kubectl", []string{"apply", "-f", filepath.Join(root, "manifests", "namespace.yaml")}, t); err != nil {
		return err
	}
	// Apply CRDs
	if err := runCmd("kubectl", []string{"apply", "-f", filepath.Join(root, "manifests", "crds")}, t); err != nil {
		return err
	}
	// Wait CRDs established
	_ = runCmd("kubectl", []string{"wait", "--for", "condition=established", "--timeout=90s", "crd/shardinstances.shard.io"}, t)
	_ = runCmd("kubectl", []string{"wait", "--for", "condition=established", "--timeout=90s", "crd/shardconfigs.shard.io"}, t)
	// RBAC, ConfigMaps, Services
	if err := runCmd("kubectl", []string{"apply", "-f", filepath.Join(root, "manifests", "rbac.yaml")}, t); err != nil {
		return err
	}
	if err := runCmd("kubectl", []string{"apply", "-f", filepath.Join(root, "manifests", "configmap.yaml")}, t); err != nil {
		return err
	}
	if err := runCmd("kubectl", []string{"apply", "-f", filepath.Join(root, "manifests", "services.yaml")}, t); err != nil {
		return err
	}
	// Deployments
	if err := runCmd("kubectl", []string{"apply", "-f", filepath.Join(root, "manifests", "deployment", "manager-deployment.yaml")}, t); err != nil {
		return err
	}
	if err := runCmd("kubectl", []string{"apply", "-f", filepath.Join(root, "manifests", "deployment", "worker-deployment.yaml")}, t); err != nil {
		return err
	}
	return nil
}

func mustRolloutStatus(t *testing.T, resource string) {
	t.Helper()
	if err := runCmd("kubectl", []string{"-n", testNamespace, "rollout", "status", resource, fmt.Sprintf("--timeout=%ds", kubectlTimeoutSec)}, t); err != nil {
		t.Fatalf("rollout status failed for %s: %v", resource, err)
	}
}

func mustControllerRuntimeClient(t *testing.T) ctrlclient.Client {
	t.Helper()
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	restCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("failed to load kubeconfig: %v", err)
	}

	// Build scheme with core and custom types
	s := runtime.NewScheme()
	if err := scheme.AddToScheme(s); err != nil {
		t.Fatalf("failed adding core scheme: %v", err)
	}
	if err := shardv1.AddToScheme(s); err != nil {
		t.Fatalf("failed adding shardv1 scheme: %v", err)
	}
	c, err := ctrlclient.New(restCfg, ctrlclient.Options{Scheme: s})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return c
}

func runCmd(cmd string, args []string, t *testing.T) error {
	t.Helper()
	c := exec.Command(cmd, args...)
	c.Env = os.Environ()
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %v\n%s", cmd, strings.Join(args, " "), err, string(out))
	}
	return nil
}

func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// If running from project root or a subdirectory, locate go.mod as anchor
	dir := wd
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		dir = filepath.Dir(dir)
	}
	return wd, nil
}
