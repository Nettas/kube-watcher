# kube-watcher
watch kube api server for events

# Init go modules:
go mod init kube-watcher

# dependencies:
go get k8s.io/client-go@latest k8s.io/apimachinery@latest \
k8s.io/client-go/tools/cache github.com/gorilla/websocket \
github.com/gin-gonic/gin

# Run backend:
go run main.go
