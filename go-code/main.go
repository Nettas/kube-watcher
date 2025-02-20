package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

var (
	upgrader   = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	clients    = make(map[*websocket.Conn]bool)
	clientsMux = sync.Mutex{}
)

// Broadcast events to all connected clients
func broadcast(msg []byte) {
	clientsMux.Lock()
	defer clientsMux.Unlock()
	for client := range clients {
		if err := client.WriteMessage(websocket.TextMessage, msg); err != nil {
			client.Close()
			delete(clients, client)
		}
	}
}

// Watch Kubernetes Pod events
func watchK8sEvents() {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)
	informer := factory.Core().V1().Pods().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			msg, _ := json.Marshal(map[string]string{"event": "ADDED", "pod": pod.Name, "namespace": pod.Namespace})
			broadcast(msg)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pod := newObj.(*v1.Pod)
			msg, _ := json.Marshal(map[string]string{"event": "UPDATED", "pod": pod.Name, "namespace": pod.Namespace})
			broadcast(msg)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			msg, _ := json.Marshal(map[string]string{"event": "DELETED", "pod": pod.Name, "namespace": pod.Namespace})
			broadcast(msg)
		},
	})

	stop := make(chan struct{})
	go informer.Run(stop)
}

// WebSocket handler
func handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket Upgrade error:", err)
		return
	}
	clientsMux.Lock()
	clients[conn] = true
	clientsMux.Unlock()

	defer func() {
		clientsMux.Lock()
		delete(clients, conn)
		clientsMux.Unlock()
		conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.GET("/ws", handleWebSocket)

	go watchK8sEvents()

	fmt.Println("Server started on :8080")
	r.Run(":8080")
}
