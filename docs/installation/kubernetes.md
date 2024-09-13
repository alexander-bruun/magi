# Kubernetes Deployment Guide

## Prerequisites

- A Kubernetes cluster up and running.
- `kubectl` configured to interact with your cluster.

## Installation Methods

### Method 1: Using a Persistent Volume Claim

1. **Create a Deployment Manifest**

    Save the following YAML as `magi-deployment.yaml`:

    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: magi-deployment
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
              image: alex-bruun/magi:latest
              volumeMounts:
                - mountPath: /data
                  name: magi-data
          volumes:
            - name: magi-data
              persistentVolumeClaim:
              claimName: magi-pvc
    ```

2. **Create a Persistent Volume Claim**

    Save the following YAML as `magi-pvc.yaml`:

    ```yaml
    apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      name: magi-pvc
    spec:
      accessModes:
        - ReadWriteOnce
      resources:
      requests:
        storage: 1Gi
    ```

3. **Apply the Manifests**

    Apply the Persistent Volume Claim and the Deployment manifest:

    ```bash
    kubectl apply -f magi-pvc.yaml
    kubectl apply -f magi-deployment.yaml
    ```

4. **Verify Deployment**

    Check the status of the deployment:

    ```bash
    kubectl get pods
    ```

    Ensure that the pods are running and the volume is mounted correctly.

### Method 2: Mounting a Host Path

1. **Create a Deployment Manifest**

    Save the following YAML as `magi-deployment-hostpath.yaml`:

    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: magi-deployment
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
              image: alex-bruun/magi:latest
              volumeMounts:
              - mountPath: /data
                name: magi-data
          volumes:
            - name: magi-data
              hostPath:
              path: /path/on/host
              type: Directory
    ```

    Replace `/path/on/host` with the actual path on the host machine that you want to mount inside the container.

2. **Apply the Manifest**

    Apply the deployment manifest:

    ```bash
    kubectl apply -f magi-deployment-hostpath.yaml
    ```

3. **Verify Deployment**

    Check the status of the deployment:

    ```bash
    kubectl get pods
    ```

    Ensure that the pods are running and the host path is mounted correctly.
