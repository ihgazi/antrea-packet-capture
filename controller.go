package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	annotationKey = "tcpdump.antrea.io"
	hostProcPath  = "/host/proc"
)

type Controller struct {
	clientset      *kubernetes.Clientset
	nodeName       string
	mu             sync.Mutex // Mutex to protect the map from concurrent informer events
	activeCaptures map[types.UID]*exec.Cmd
}

func NewController(cs *kubernetes.Clientset, node string) *Controller {
	return &Controller{
		clientset:      cs,
		nodeName:       node,
		activeCaptures: make(map[types.UID]*exec.Cmd),
	}
}

func (c *Controller) handlePodUpdate(newObj interface{}) {
	pod, ok := newObj.(*corev1.Pod)
	if !ok {
		return
	}

	// Only handle Pods in the current node
	if pod.Spec.NodeName != c.nodeName {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	nStr, annotated := pod.Annotations[annotationKey]
	_, running := c.activeCaptures[pod.UID]

	// Logic Tree:
	// A. Annotated + Not Running -> Start Capture
	// B. Not Annotated + Running -> Stop Capture
	// C. Annotated + Running ->  Handle changes to N

	if annotated && !running {
		c.startCapture(pod, nStr)
	} else if !annotated && running {
		c.stopCapture(pod.UID, pod.Name)
	} else if annotated && running {
		c.stopCapture(pod.UID, pod.Name)
		c.startCapture(pod, nStr)
	}
}

func (c *Controller) handlePodAdd(obj interface{}) {
	c.handlePodUpdate(obj)
}

func (c *Controller) handlePodDelete(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, running := c.activeCaptures[pod.UID]; running {
		log.Printf("Pod %s deleted, stopping capture\n", pod.Name)
		c.stopCapture(pod.UID, pod.Name)
	}
}

func (c *Controller) startCapture(pod *corev1.Pod, nStr string) {
	n, err := strconv.Atoi(nStr)
	if err != nil || n <= 0 {
		log.Printf("Invalid number of packets for pod %s: %v\n", pod.Name, err)
		return
	}

	podIP := pod.Status.PodIP
	if podIP == "" {
		log.Printf("Pod %s has no IP assigned, cannot start capture\n", pod.Name)
		return
	}

	pcapFile := fmt.Sprintf("/captures/capture-%s.pcap", pod.Name)

	cmd := exec.Command("tcpdump", "-C", "1", "-W", strconv.Itoa(n), "-w", pcapFile, "-i", "any", fmt.Sprintf("host %s", podIP))

	// Enable tcpdump logs for debugging
	/**
	stderr, _ := cmd.StderrPipe()
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("tcpdump stderr [%s]: %s\n", pod.Name, scanner.Text())
		}
	}()**/

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start tcpdump for pod %s: %v\n", pod.Name, err)
		return
	}

	c.activeCaptures[pod.UID] = cmd
	log.Printf("Started tcpdump for pod %s (N=%d)\n", pod.Name, n)

	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("tcpdump for pod %s exited: %v\n", pod.Name, err)
		}
	}()
}

func (c *Controller) stopCapture(uid types.UID, podName string) {
	cmd, exists := c.activeCaptures[uid]
	if !exists {
		return
	}

	// Kill the tcpdump process
	if err := cmd.Process.Kill(); err != nil {
		log.Printf("Failed to kill tcpdump for pod %s: %v\n", podName, err)
	}
	delete(c.activeCaptures, uid)

	// Cleanup pcap files created by tcpdump
	files, _ := os.ReadDir("/captures")
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "capture-"+podName) {
			os.Remove("/captures/" + f.Name())
		}
	}

	log.Printf("Stopped capture and cleaned up files for pod %s\n", podName)
}
