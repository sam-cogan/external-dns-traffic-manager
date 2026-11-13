package tests

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func TestSingleClusterAzureWorkloadIdentityConfig(t *testing.T) {
	file, err := os.Open("../examples/single-cluster/external-dns.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	decoder := yaml.NewYAMLOrJSONDecoder(file, 4096)
	var namespace corev1.Namespace
	var config corev1.ConfigMap
	var deploy appsv1.Deployment
	for _, obj := range []any{&namespace, &config, &deploy} {
		if err := decoder.Decode(obj); err != nil {
			t.Fatal(err)
		}
	}
	var azureConfig struct {
		WorkloadIdentity bool `json:"useWorkloadIdentityExtension"`
	}
	if err := json.Unmarshal([]byte(config.Data["azure.json"]), &azureConfig); err != nil || !azureConfig.WorkloadIdentity {
		t.Fatalf("invalid workload identity configuration: %v", err)
	}
	var azure *corev1.Container
	for i := range deploy.Spec.Template.Spec.Containers {
		container := &deploy.Spec.Template.Spec.Containers[i]
		if container.Name == "external-dns-azure" {
			azure = container
			break
		}
	}
	if azure == nil {
		t.Fatal("Azure provider container not found")
	}
	var configFile string
	for _, arg := range azure.Args {
		if strings.HasPrefix(arg, "--azure-config-file=") {
			configFile = strings.TrimPrefix(arg, "--azure-config-file=")
		}
	}
	for _, mount := range azure.VolumeMounts {
		if configFile != mount.MountPath+"/azure.json" || !mount.ReadOnly {
			continue
		}
		for _, volume := range deploy.Spec.Template.Spec.Volumes {
			if volume.Name == mount.Name && volume.ConfigMap != nil && volume.ConfigMap.Name == config.Name {
				return
			}
		}
	}
	t.Fatal("Azure configuration file is not mounted from its ConfigMap")
}
