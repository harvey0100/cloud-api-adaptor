// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

type PatchLabel struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

var Action string

// Adds the worker label to all workers nodes in a given cluster
func AddNodeRoleWorkerLabel(ctx context.Context, clusterName string, cfg *envconf.Config) error {
	fmt.Printf("Adding worker label to nodes belonging to: %s\n", clusterName)
	client, err := cfg.NewClient()
	if err != nil {
		return err
	}

	nodelist := &corev1.NodeList{}
	if err := client.Resources().List(ctx, nodelist); err != nil {
		return err
	}
	// Use full path to avoid overwriting other labels (see RFC 6902)
	payload := []PatchLabel{{
		Op: "add",
		// "/" must be written as ~1 (see RFC 6901)
		Path:  "/metadata/labels/node.kubernetes.io~1worker",
		Value: "",
	}}
	payloadBytes, _ := json.Marshal(payload)
	workerStr := clusterName + "-worker"
	for _, node := range nodelist.Items {
		if strings.Contains(node.Name, workerStr) {
			if err := client.Resources().Patch(ctx, &node, k8s.Patch{PatchType: types.JSONPatchType, Data: payloadBytes}); err != nil {
				return err
			}
		}

	}
	return nil
}

func GetPodLog(ctx context.Context, client klient.Client, pod corev1.Pod) (string, error) {
	clientset, err := kubernetes.NewForConfig(client.RESTConfig())
	if err != nil {
		return "", err
	}

	req := clientset.CoreV1().Pods(pod.ObjectMeta.Namespace).GetLogs(pod.ObjectMeta.Name, &corev1.PodLogOptions{})
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer podLogs.Close()
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func DeletePod(ctx context.Context, client klient.Client, pod *corev1.Pod, deleteDuration *time.Duration) error {
	duration := 1 * time.Minute
	if deleteDuration == nil {
		deleteDuration = &duration
	}
	if err := client.Resources().Delete(ctx, pod); err != nil {
		return err
	}
	log.Infof("Deleting pod %s...", pod.Name)
	if err := wait.For(conditions.New(
		client.Resources()).ResourceDeleted(pod),
		wait.WithInterval(5*time.Second),
		wait.WithTimeout(*deleteDuration)); err != nil {
		return err
	}
	log.Infof("Pod %s has been successfully deleted within %.0fs", pod.Name, deleteDuration.Seconds())
	return nil
}
