# Cloud API Adaptor (CAA) on Google Cloud Compute (GCP)

This documentation will walk you through setting up Cloud API Adaptor (CAA) on
Google Compute Engine (GCE). We will build the Pod VM image, CCA dev image and
experiment on a local libvirt cluster.

## Disclaimer

> [!WARNING]
> Those instructions assume you are using CAA with kata-container main branch,
> instead of CCv0. At this moment this setup will not be able to manage the
> continer, only the virtual machine, since we are still working on the
> snappshotter port.

> [!NOTE]
>  So far, this effort is assuming you are running a local k8s cluster
> and will trigger VMs at Google Cloud. Next step would be to integrate with GKE
> (Google Kubernetes Engine).

## Install and configure requirements

### Create a GCP account and project

First of all, visit [GCP website](https://cloud.google.com/) and create your
account and a project. Keep the project name in mind, we will use it.

Store the project id at this variable:

```bash
$ export GCP_PROJECT_ID="REPLACE_ME" # Replace with your project id
```

### Install `gcloud` cli and run `gcloud init`

You will need to install `gcloud` command line to interact with the cloud in
different operations. Follow the [official instructions](https://cloud.google.com/sdk/docs/install)

After that, bootstrap gcloud configuration with the Getting started workflow,
follow the instructions above to see details about this workflow, but basically
you will need to run:

```bash
$ gcloud init
 ```

### Create packer service account

Create 'packer' service account with compute instance admin role:

```bash
$ gcloud iam service-accounts create packer \
 --project ${GCP_PROJECT_ID} \
  --description="Packer Service Account" \
  --display-name="Packer Service Account"
```

```bash
$ gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} \
  --member=serviceAccount:packer@${GCP_PROJECT_ID}.iam.gserviceaccount.com \
  --role=roles/compute.instanceAdmin.v1
```

```bash
$ gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} \
  --member=serviceAccount:packer@${GCP_PROJECT_ID}.iam.gserviceaccount.com \
  --role=roles/iam.serviceAccountUser
```

Create an application access token and store it on a variable:

```bash
$ gcloud iam service-accounts keys create ${HOME}/.config/gcloud/packer_application_key.json \
  --iam-account=packer@${GCP_PROJECT_ID}.iam.gserviceaccount.com
```

```bash
export GOOGLE_APPLICATION_CREDENTIALS=${HOME}/.config/gcloud/packer_application_key.json
```

### Allow ingress traffic for the agent-protocol-forwarder

Update the `default` (global)
network and add a firewall rule that allows incoming TCP connections over port
15150 from your workstation external IP address.

```bash
gcloud compute firewall-rules create allow-tcp-15150 --network default --allow tcp:15150 --source-ranges  [YOUR_EXTERNAL_IP]/32
```

### Configure a few more variables

```bash
export GCP_ZONE="REPLACE_ME" # e.g. "us-west1-a"
export GCP_MACHINE_TYPE="REPLACE_ME" # default is "e2-medium"
export GCP_NETWORK="REPLACE_ME" # default is "default"
export CLOUD_PROVIDER=gcp
```

### Prepare the Kubernetes cluster

1. **Create a cluster**:

Ensure your cluster is prepared for PeerPods workloads. For detailed steps on deploying the CoCo operator, CC runtime CRD, and the cloud-api-adaptor daemonset, consult the [Install Guide](https://github.com/cfircohen/cloud-api-adaptor/blob/peerpod_gcp/install/README.md#deploy-the-coco-operator-cc-runtime-crd-and-the-cloud-api-adaptor-daemonset).

2. **Deploy the CoCo Operator and custom objects**:

You cluster need to be ready to accept PeerPods workloads. Follow the [Deploy the CoCo operator, CC runtime CRD and the cloud-api-adaptor daemonset](https://github.com/cfircohen/cloud-api-adaptor/blob/peerpod_gcp/install/README.md#deploy-the-coco-operator-cc-runtime-crd-and-the-cloud-api-adaptor-daemonset) section for details.

### Install packer binary (Optional)

Follow
[official instructions](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli) to install `packer` command line.
> [!NOTE]
> Only necessary if you plan to build the Pod VM image locally.

### Configure a local registry (Optional)


For development, consider setting up a local Docker registry. See the
[Local Registry Guide](/docs/local-registry.md) for setup details.

## Build and upload the Pod VM image

Lets create a custom GCP VM image based on Ubuntu 20.04 having kata-agent,
agent-protocol-forwarder and others dependencies needed to manage the container
and communicate with others Kata components.

This image will be later uploaded to the Cloud bucket and available to launch
the VMs.

There are two methods to build this image. You can build it using docker or
locally:

### Build using docker

> [!NOTE]
> TODO: This method is not working at the moment.

```bash
cd image
DOCKER_BUILDKIT=1 docker build -t gcp \
  --secret id=APPLICATION_KEY,src=${GOOGLE_APPLICATION_CREDENTIALS} \
  --build-arg GCP_PROJECT_ID=${GCP_PROJECT_ID} \
  --build-arg GCP_ZONE=${GCP_ZONE} \
  -f Dockerfile .
```

### Build locally

First, make sure you have installed the `packer` binary if you are building
locally. See *"Install packer binary (Optional)"* section above.

```bash
cd image
PODVM_DISTRO=ubuntu make image && cd -
```

> [!NOTE]
> TODO: Consider renaming this target to `make podvm-image`, for consistency?

## Build the cloud-api-adaptor image

To build and publish the cloud-api-adaptor image using your current working
directory, perform the following steps. Adjust the `registry` variable to point
to your preferred registry:

```bash
export registry=192.168.122.1:5000 # Replace with your registry address
export RELEASE_BUILD=true
make image
```

## Deploy Cloud API Adaptor

1. **Adjust the overlay**:

Update [install/overlays/gcp/kustomization.yaml](/install/overlays/gcp/kustomization.yaml) with the required fields:

```
images:
- name: cloud-api-adaptor
  newName: 192.168.122.1:5000/cloud-api-adaptor # change image if needed
  newTag: 47dcc2822b6c2489a02db83c520cf9fdcc833b3f-dirty # change if needed
  ...
configMapGenerator:
- name: peer-pods-cm
  namespace: confidential-containers-system
  literals:
  - CLOUD_PROVIDER="gcp" # leave as is.
  - PODVM_IMAGE_NAME="" # set from step 1 above.
  - GCP_PROJECT_ID="" # set
  - GCP_ZONE="" # set
  - GCP_MACHINE_TYPE="e2-medium" # defaults to e2-medium
  - GCP_NETWORK="global/networks/default" # leave as is.
  ...
secretGenerator:
  ...
- name: peer-pods-secret
  namespace: confidential-containers-system
  files:
  - GCP_CREDENTIALS
```

Ensure that the peer-pods-secret has the required GCP credentials. You can
utilize the credentials already used by Packer. To do this, simply copy the
application credentials file from `$GCP_CREDENTIALS` to the location specified
in the secretGenerator configuration for GCP_CREDENTIALS.

2. **Apply the objects**:

```bash
$ kubectl apply -k install/overlays/gcp/
$ kubectl get pods -n confidential-containers-system
NAME                                              READY   STATUS    RESTARTS   AGE
cc-operator-controller-manager-546574cf87-b69pb   2/2     Running   0          7d10h
cc-operator-daemon-install-mfjbj                  1/1     Running   0          7d10h
cc-operator-pre-install-daemon-g2jsj              1/1     Running   0          7d10h
cloud-api-adaptor-daemonset-5w8nw                 1/1     Running   0          7s
```

## Deploy a sample workload

Deploy the `sample_busybox.yaml` (see [libvirt/README.md](/libvirt/README.md)):

```
$ kubectl apply -f sample_busybox.yaml
pod/busybox created
```

Examine your GCP console, under "Compute Engine", "VM instances" you should see the new POD instance running.
Examine `kubectl logs` and verify the tunnel to the podvm was established successfully.

## Cleanup

> [!NOTE]
> Todo
