package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	// Set up routes
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/logs", handleLogs)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("MCP K8s Log Tool running on port %s\n", port)
	log.Printf("Server starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, enableCORS(http.DefaultServeMux)))
}

// handleRoot provides API documentation
func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>MCP K8s Log Tool</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        code { background: #f4f4f4; padding: 2px 4px; }
        .endpoint { background: #e8f4fd; padding: 15px; margin: 10px 0; border-radius: 5px; }
    </style>
</head>
<body>
    <h1>MCP K8s Log Tool</h1>
    <p>A simple tool to fetch Kubernetes pod logs via HTTP API</p>
    
    <div class="endpoint">
        <h3>GET /logs</h3>
        <p>Fetch pod logs from Kubernetes cluster</p>
        <p><strong>Parameters:</strong></p>
        <ul>
            <li><code>namespace</code> - Kubernetes namespace (required)</li>
            <li><code>pod</code> - Pod name (required)</li>
            <li><code>container</code> - Container name (optional)</li>
            <li><code>lines</code> - Number of lines to fetch (optional, default: 100)</li>
            <li><code>follow</code> - Stream logs (optional, true/false)</li>
        </ul>
        <p><strong>Example:</strong></p>
        <code>/logs?namespace=default&pod=my-pod&lines=50</code>
    </div>
    
    <div class="endpoint">
        <h3>GET /health</h3>
        <p>Health check endpoint</p>
    </div>
</body>
</html>
`)
}

// handleHealth provides health check endpoint
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "healthy", "timestamp": "%s"}`+"\n", time.Now().Format(time.RFC3339))
}

// handleLogs fetches and returns Kubernetes pod logs
func handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	namespace := r.URL.Query().Get("namespace")
	pod := r.URL.Query().Get("pod")
	container := r.URL.Query().Get("container")
	linesStr := r.URL.Query().Get("lines")
	followStr := r.URL.Query().Get("follow")

	if namespace == "" || pod == "" {
		http.Error(w, "namespace and pod parameters are required", http.StatusBadRequest)
		return
	}

	// Parse lines parameter
	var lines *int64
	if linesStr != "" {
		if l, err := strconv.ParseInt(linesStr, 10, 64); err == nil && l > 0 {
			lines = &l
		}
	}
	if lines == nil {
		defaultLines := int64(100)
		lines = &defaultLines
	}

	// Parse follow parameter
	follow := followStr == "true"

	// Get Kubernetes client
	clientset, err := getK8sClient()
	if err != nil {
		log.Printf("Failed to create Kubernetes client: %v", err)
		http.Error(w, "Failed to connect to Kubernetes cluster", http.StatusInternalServerError)
		return
	}

	// Check if pod exists
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = clientset.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		log.Printf("Pod %s not found in namespace %s: %v", pod, namespace, err)
		http.Error(w, fmt.Sprintf("Pod '%s' not found in namespace '%s'", pod, namespace), http.StatusNotFound)
		return
	}

	// Prepare log options
	logOptions := &corev1.PodLogOptions{
		TailLines: lines,
		Follow:    follow,
	}
	if container != "" {
		logOptions.Container = container
	}

	// Get logs
	req := clientset.CoreV1().Pods(namespace).GetLogs(pod, logOptions)
	logStream, err := req.Stream(ctx)
	if err != nil {
		log.Printf("Failed to get logs for pod %s/%s: %v", namespace, pod, err)
		http.Error(w, fmt.Sprintf("Failed to fetch logs: %v", err), http.StatusInternalServerError)
		return
	}
	defer logStream.Close()

	// Set appropriate headers
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if follow {
		w.Header().Set("Transfer-Encoding", "chunked")
	}

	// Stream logs to response
	if _, err := io.Copy(w, logStream); err != nil {
		log.Printf("Error streaming logs: %v", err)
	}
}

// getK8sClient creates a Kubernetes client using kubeconfig or in-cluster config
func getK8sClient() (*kubernetes.Clientset, error) {
	// Try in-cluster config first (when running inside Kubernetes)
	if config, err := rest.InClusterConfig(); err == nil {
		return kubernetes.NewForConfig(config)
	}

	// Fall back to kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	if kubeconfig == "" {
		return nil, fmt.Errorf("no kubeconfig found")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	return kubernetes.NewForConfig(config)
}

// enableCORS adds CORS headers to allow browser access
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
