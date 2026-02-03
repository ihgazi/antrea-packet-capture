package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// Get Node Name from environment
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		panic("NODE_NAME environment variable must be set")
	}

	// Initialize Kubernetes Client
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Setup Informer Factory
	factory := informers.NewSharedInformerFactory(clientset, time.Second*30)
	podInformer := factory.Core().V1().Pods().Informer()

	// Initialize Controller Logic
	ctrl := NewController(clientset, nodeName)

	// Setup Event Handlers
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: ctrl.handlePodAdd,
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*corev1.Pod)
			newPod := newObj.(*corev1.Pod)

			// Only handle Pods if the ResourceVersion or Annotations changed
			if oldPod.ResourceVersion == newPod.ResourceVersion {
				return
			}
			ctrl.handlePodUpdate(newObj)
		},
		DeleteFunc: ctrl.handlePodDelete,
	})

	// Start Informer and wait for stop signal
	stopCh := make(chan struct{})
	defer close(stopCh)

	log.Printf("Starting PacketCapture Controller on node: %s\n", nodeName)
	factory.Start(stopCh)

	// Wait for SIGINT or SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Shutting down...")
}
