# Kubernetes Deployment Guide

Deploy Magi on Kubernetes for high availability and scalability.

## Prerequisites

- Kubernetes cluster (1.19+)
- `kubectl` configured to access your cluster
- Persistent storage provisioner (optional but recommended)
- At least 512MB RAM per pod

## Quick Start

### Single Command Deployment

```bash
kubectl apply -f https://raw.githubusercontent.com/alexander-bruun/magi/main/k8s/deployment.yaml
```

## Manual Deployment

### Method 1: Using Persistent Volume Claim (Recommended)

This method uses dynamic provisioning for persistent storage.

#### 1. Create Namespace

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: magi
```

Apply:
```bash
kubectl apply -f namespace.yaml
```

#### 2. Create Persistent Volume Claim

```yaml
# pvc.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: magi-data
  namespace: magi
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: standard  # Change to your storage class
```

Apply:
```bash
kubectl apply -f pvc.yaml
```

#### 3. Create Deployment

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: magi
  namespace: magi
  labels:
    app: magi
spec:
  replicas: 1
  selector:
    matchLabels:
      app: magi
  template:
    metadata:
      labels:
        app: magi
    spec:
      containers:
      - name: magi
        image: alexbruun/magi:latest
        imagePullPolicy: Always
        ports:
        - containerPort: 3000
          name: http
          protocol: TCP
        env:
        - name: MAGI_DATA_DIR
          value: /data/magi
        - name: PORT
          value: "3000"
        - name: TZ
          value: "UTC"
        volumeMounts:
        - name: magi-data
          mountPath: /data/magi
        - name: manga-storage
          mountPath: /data/manga
          readOnly: true
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "2Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /
            port: 3000
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /
            port: 3000
          initialDelaySeconds: 10
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 3
      volumes:
      - name: magi-data
        persistentVolumeClaim:
          claimName: magi-data
      - name: manga-storage
        hostPath:
          path: /mnt/manga  # Change to your manga path
          type: Directory
```

Apply:
```bash
kubectl apply -f deployment.yaml
```

#### 4. Create Service

```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: magi
  namespace: magi
  labels:
    app: magi
spec:
  type: ClusterIP
  ports:
  - port: 3000
    targetPort: 3000
    protocol: TCP
    name: http
  selector:
    app: magi
```

Apply:
```bash
kubectl apply -f service.yaml
```

#### 5. Create Ingress (Optional)

For external access with a domain name:

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: magi
  namespace: magi
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/proxy-body-size: "100m"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - magi.example.com
    secretName: magi-tls
  rules:
  - host: magi.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: magi
            port:
              number: 3000
```

Apply:
```bash
kubectl apply -f ingress.yaml
```

### Method 2: Using HostPath Volumes

Suitable for single-node clusters or testing.

```yaml
# deployment-hostpath.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: magi
  namespace: magi
spec:
  replicas: 1
  selector:
    matchLabels:
      app: magi
  template:
    metadata:
      labels:
        app: magi
    spec:
      containers:
      - name: magi
        image: alexbruun/magi:latest
        ports:
        - containerPort: 3000
        volumeMounts:
        - name: magi-data
          mountPath: /data/magi
        - name: manga-storage
          mountPath: /data/manga
          readOnly: true
      volumes:
      - name: magi-data
        hostPath:
          path: /mnt/kubernetes/magi
          type: DirectoryOrCreate
      - name: manga-storage
        hostPath:
          path: /mnt/manga
          type: Directory
```

Apply:
```bash
kubectl apply -f deployment-hostpath.yaml
```

### Method 3: Using NFS Storage

For shared storage across multiple nodes:

#### 1. Create NFS Persistent Volume

```yaml
# pv-nfs.yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: magi-nfs-pv
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadWriteMany
  nfs:
    server: nfs-server.example.com
    path: /export/magi
  mountOptions:
    - hard
    - nfsvers=4.1
```

#### 2. Create PVC

```yaml
# pvc-nfs.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: magi-data
  namespace: magi
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 10Gi
  volumeName: magi-nfs-pv
```

Apply:
```bash
kubectl apply -f pv-nfs.yaml
kubectl apply -f pvc-nfs.yaml
```

## Verification

Check deployment status:

```bash
# Check all resources in magi namespace
kubectl get all -n magi

# Check pod logs
kubectl logs -f deployment/magi -n magi

# Check pod details
kubectl describe pod -l app=magi -n magi

# Check persistent volumes
kubectl get pv
kubectl get pvc -n magi
```

## Accessing Magi

### Port Forward (Testing)

```bash
kubectl port-forward -n magi service/magi 3000:3000
```

Then access: `http://localhost:3000`

### NodePort Service

```yaml
# service-nodeport.yaml
apiVersion: v1
kind: Service
metadata:
  name: magi
  namespace: magi
spec:
  type: NodePort
  ports:
  - port: 3000
    targetPort: 3000
    nodePort: 30000  # Choose port 30000-32767
  selector:
    app: magi
```

Access via: `http://[node-ip]:30000`

### LoadBalancer Service

For cloud providers:

```yaml
# service-loadbalancer.yaml
apiVersion: v1
kind: Service
metadata:
  name: magi
  namespace: magi
spec:
  type: LoadBalancer
  ports:
  - port: 80
    targetPort: 3000
  selector:
    app: magi
```

Get external IP:
```bash
kubectl get service magi -n magi
```

## Configuration

### ConfigMap for Environment Variables

```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: magi-config
  namespace: magi
data:
  MAGI_DATA_DIR: "/data/magi"
  PORT: "3000"
  TZ: "America/New_York"
```

Reference in deployment:

```yaml
spec:
  containers:
  - name: magi
    envFrom:
    - configMapRef:
        name: magi-config
```

### Secrets for Sensitive Data

```yaml
# secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: magi-secrets
  namespace: magi
type: Opaque
stringData:
  admin-password: "change-me"
```

## Updating Magi

### Rolling Update

```bash
# Update image to latest
kubectl set image deployment/magi magi=alexbruun/magi:latest -n magi

# Or edit deployment
kubectl edit deployment magi -n magi

# Watch rollout status
kubectl rollout status deployment/magi -n magi
```

### Rollback

```bash
# View rollout history
kubectl rollout history deployment/magi -n magi

# Rollback to previous version
kubectl rollout undo deployment/magi -n magi

# Rollback to specific revision
kubectl rollout undo deployment/magi --to-revision=2 -n magi
```

## Scaling

```bash
# Scale to multiple replicas (if using ReadWriteMany storage)
kubectl scale deployment magi --replicas=3 -n magi

# Auto-scaling (HPA)
kubectl autoscale deployment magi \
  --cpu-percent=80 \
  --min=1 \
  --max=5 \
  -n magi
```

> [!WARNING]
> Magi uses SQLite which doesn't support concurrent writes from multiple pods. Keep replicas=1 unless using a different database backend.

## Troubleshooting

### Pod Not Starting

```bash
# Check pod status
kubectl get pods -n magi

# Describe pod for events
kubectl describe pod -l app=magi -n magi

# Check logs
kubectl logs -f deployment/magi -n magi

# Check previous container logs (if restarting)
kubectl logs deployment/magi -n magi --previous
```

### Storage Issues

```bash
# Check PVC status
kubectl get pvc -n magi

# Describe PVC
kubectl describe pvc magi-data -n magi

# Check PV
kubectl get pv
```

### Network Issues

```bash
# Test service connectivity from another pod
kubectl run -it --rm debug --image=busybox --restart=Never -n magi -- wget -qO- http://magi:3000

# Check service endpoints
kubectl get endpoints magi -n magi
```

## Monitoring

### Resource Usage

```bash
# View resource usage
kubectl top pod -n magi
kubectl top node
```

### Logs

```bash
# Stream logs
kubectl logs -f deployment/magi -n magi

# Last 100 lines
kubectl logs deployment/magi -n magi --tail=100

# Logs from all pods
kubectl logs -l app=magi -n magi --all-containers=true
```

## Complete Manifest

All-in-one manifest:

```yaml
# magi-complete.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: magi
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: magi-data
  namespace: magi
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: magi
  namespace: magi
spec:
  replicas: 1
  selector:
    matchLabels:
      app: magi
  template:
    metadata:
      labels:
        app: magi
    spec:
      containers:
      - name: magi
        image: alexbruun/magi:latest
        ports:
        - containerPort: 3000
        env:
        - name: MAGI_DATA_DIR
          value: /data/magi
        volumeMounts:
        - name: magi-data
          mountPath: /data/magi
      volumes:
      - name: magi-data
        persistentVolumeClaim:
          claimName: magi-data
---
apiVersion: v1
kind: Service
metadata:
  name: magi
  namespace: magi
spec:
  type: ClusterIP
  ports:
  - port: 3000
    targetPort: 3000
  selector:
    app: magi
```

Deploy:
```bash
kubectl apply -f magi-complete.yaml
```

## Cleanup

Remove all Magi resources:

```bash
# Delete namespace (removes everything)
kubectl delete namespace magi

# Or delete individual resources
kubectl delete deployment magi -n magi
kubectl delete service magi -n magi
kubectl delete pvc magi-data -n magi
kubectl delete pv magi-nfs-pv  # If using NFS
```

## Next Steps

- [Configure your first library](../usage/getting_started.md)
- [Set up ingress with TLS](../usage/configuration.md)
- [Monitor with Prometheus/Grafana](../usage/troubleshooting.md)
