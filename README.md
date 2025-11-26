# 🎉 Node-Process-Exporter

---
这是一个监控主机进程的exporter，用于分析主机进行的资源使用情况；通常在节点资源突然暴增时能够通过大盘快速定位到相应的 process。

## 运行 exporter

### 本地运行
```bash
./process -port=9002
```

### Docker 运行
```bash
docker run -p 9002:9002 cairry/node-process-exporter:latest
```

### Kubernetes 运行
``` 
apiVersion: apps/v1
kind: DaemonSet
metadata:
  labels:
    app: node-process-exporter
  name: node-process-exporter
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: node-process-exporter
  template:
    metadata:
      labels:
        app: node-process-exporter
    spec:
      containers:
      - image: cairry/node-process-exporter:latest
        imagePullPolicy: IfNotPresent
        name: node-process-exporter
        args: ["-port=9002"]
        ports:
        - containerPort: 9002
          hostPort: 9002
          protocol: TCP
        resources:
        limits:
          cpu: "1"
          memory: 1Gi
        requests:
          cpu: 250m
          memory: 512Mi
        securityContext:
          privileged: true

      hostIPC: true
      hostNetwork: true
      hostPID: true
      restartPolicy: Always

---
apiVersion: v1
kind: Service
metadata:
  name: node-process-exporter
  namespace: kube-system
spec:
  ports:
  - port: 9002
    protocol: TCP
    targetPort: 9002
  selector:
    app: node-process-exporter
  sessionAffinity: None
  type: ClusterIP
```
## Metric 格式
``` 
# HELP node_process_cpu_usage_percent Process CPU usage percentage
# TYPE node_process_cpu_usage_percent gauge
node_process_cpu_usage_percent{cmd="/app/process",name="process",pid="1008428",user="root"} 1.625085769769547

# HELP node_process_memory_usage_percent Process memory usage percentage
# TYPE node_process_memory_usage_percent gauge
node_process_memory_usage_percent{cmd="/app/process",name="process",pid="1008428",user="root"} 0.03518591615313425

# HELP node_process_open_files_count Number of open files by the process
# TYPE node_process_open_files_count gauge
node_process_open_files_count{cmd="/app/process",name="process",pid="1008428",user="root"} 9

# HELP node_process_read_bytes_total Total number of bytes read by the process.
# TYPE node_process_read_bytes_total counter
node_process_read_bytes_total{cmd="/app/process",name="process",pid="1322556",user="root"} 0

# HELP node_process_write_bytes_total Total number of bytes written by the process.
# TYPE node_process_write_bytes_total counter
node_process_write_bytes_total{cmd="/app/process",name="process",pid="1322556",user="root"} 0
```

## Prometheus 配置
``` 
    - job_name: node-process-exporter
      honor_timestamps: true
      scrape_interval: 30s
      scrape_timeout: 10s
      metrics_path: /metrics
      scheme: http
      relabel_configs:
      - source_labels: [__address__]
        separator: ;
        regex: (.*):9002
        replacement: $1
        action: keep
      kubernetes_sd_configs:
      - role: endpoints
```

## 示图
导入 ./dashboard.json

![img.png](img.png)