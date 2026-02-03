# Antrea Packet Capture

A Kubernetes controller for capturing network packets from pods on demand using `tcpdump`.

---

## Overview

The packet capture controller runs as a DaemonSet and watches Pods on each Node. When a pod is annotated with `tcpdump.antrea.io: "<N>"`, the controller starts a `tcpdump` process to capture packets for that pod. The capture is stored in rotating files (1 MB each, max N files). If the annotation is removed, the capture stops and the files are deleted.

---

## Prerequisites

- [Go](https://golang.org/) 1.18+
- [Docker](https://www.docker.com/)
- [Kind](https://kind.sigs.k8s.io/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)

---

## Quick Start

```bash
# Create Kind cluster with Antrea
kind create cluster --config kind-config.yaml
helm repo add antrea https://charts.antrea.io && helm repo update
helm install antrea antrea/antrea --n kube-system

# Build image and deploy DaemonSet
make docker-build
make kind-load
make deploy

# Deploy a test pod for generating traffic
make test-pod

# Annotate the pod to start packet capture
kubectl annotate pod test-pod tcpdump.antrea.io="5"

# Verify packet capture
CAPTURE_POD=$(kubectl get pods -n kube-system -l app=packet-capture \
  --field-selector spec.nodeName=$(kubectl get pod -n kube-system traffic-generator -o jsonpath='{.spec.nodeName}') \
  -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n kube-system $CAPTURE_POD -- ls /captures

# Stop capture and clean up files
kubectl annotate pod traffic-generator tcpdump.antrea.io-
```

## Implementation Details

- Controller Architecture: Runs as a DaemonSet, with one instance per node. The controller is built using the Informer pattern and uses a `fieldSelector` to watch only those Pods scheduled on its specific node.

- Annotation Trigger: Starts a packet capture when a pod is annotated with `tcpdump.antrea.io: "<N>"`, where N is the number of rotating pcap files.

- Tcpdump Management: Launches a `tcpdump` process in the host network namespace with the following parameters: `tcpdump -i any -C 1 -W <N> -w /captures/captres-<pod-name>.pcap host <pod-ip>`.

- Cleanup: Stops the `tcpdump` process using context cancellation signal and performs a glob-based search to delete all rotated .pcap files, when the anonotatin is removed.

