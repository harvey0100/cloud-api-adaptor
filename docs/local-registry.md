# Local Image Registry Configuration

> [!WARNING]
> The local registry setup is particularly useful for development and
> testing. For production environments, it's recommended to use secure, managed
> registries.

This guide outlines the steps to configure a local Docker registry for a Kubernetes environment. This setup is useful for development and testing purposes, allowing you to use custom or local Docker images without the need to fetch images from the Internet.

## Prerequisites

- Docker installed on your machine.
- A Kubernetes cluster, such as one set up using Kubeadm.

## Step 1: Running a Local Docker Registry

Start by setting up a local Docker registry. This will act as a private storage
for your Docker images.

1. **Stop and remove any existing registry container (if present)**:

    ```bash
    docker stop registry
    docker rm registry
    ```

2. **Run the Docker registry container**:

    ```bash
    docker run -d -p 5000:5000 --restart=always --name registry registry:2.7.0
    ```

    This command starts a new Docker registry container that listens on port 5000.

## Step 2: Configuring you cluster to use the local registry

After setting up the local Docker registry, you'll need to configure your
Kubernetes nodes to use this registry.

1. **SSH into your Kubernetes worker node**:

    Replace `<node-name>` with the name of your node.

    ```bash
    kcli ssh <node-name>
    ```

2. **Edit the Containerd configuration**:

    Open the Containerd configuration file in an editor:

    ```bash
    sudo vim /etc/containerd/config.toml
    ```

    Add the following configuration to allow pulling images from the local registry:

    ```toml
    [plugins."io.containerd.grpc.v1.cri".registry]
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."192.168.122.1:5000"]
          endpoint = ["http://192.168.122.1:5000"]
      [plugins."io.containerd.grpc.v1.cri".registry.configs]
        [plugins."io.containerd.grpc.v1.cri".registry.configs."192.168.122.1:5000".tls]
          insecure_skip_verify = true
    ```

3. **Restart Containerd**:

    Apply the changes by restarting Containerd:

    ```bash
    sudo systemctl restart containerd
    ```

With these steps, your Kubernetes cluster is now configured to use the local Docker registry, allowing your worker nodes to pull images from it.