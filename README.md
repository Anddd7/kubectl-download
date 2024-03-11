# kubectl-download

Output and download any kubernetes resources in to named files.

It simplifies the process of `kubectl get pod nginx -oyaml > nginx.yaml`, you don't need pipe and name a file manually. Just `kubectl download pod nginx`, everything in "pods_my-pod.yaml".

## How to use

```sh
kubectl donwload pod
kubectl download deploy my-deploy
kubectl download ingress my-ingress -n my-namespace
```

## Install

### krew custom index

Add the index

- `kubectl krew index add download https://github.com/Anddd7/kubectl-download.git`

Install the plugin

- `kubectl krew install download/download`

### - krew install directly

Install vai custom manifest

- `kubectl krew install --manifest-url https://raw.githubusercontent.com/Anddd7/kubectl-download/main/plugins/download.yaml`

### manual install

Enter the latest release and download the binary for your platform

- `curl -o kubectl-download https://github.com/Anddd7/kubectl-download/releases/download/v1.0.0/kubectl-download_v1.0.0_linux_arm64.tar.gz`

Move it the krew bin directory or any directory in your PATH

- `mv kubectl-download ~/.krew/bin`

## Completion

Register completion for plugins of kubectl

```sh
# download completion shell script
curl -o kubectl_complete-download https://raw.githubusercontent.com/Anddd7/kubectl-download/main/completion/kubectl_complete-download

# add execute permission
chmod +x kubectl_complete-download

# move to krew bin directory
mv kubectl_complete-download ~/.krew/bin
```

Or using [plugin-completion](https://github.com/marckhouzam/kubectl-plugin_completion) ...

## TODO

- [x] docs
- [x] support krew install (custom index)
- [ ] filter server fields (e.g. managed fields)
  - drop specific fields in yaml file
  - or delete key from map interface before marshalling
- [x] github workflow
- [x] renovate
- [x] zsh complete
  - [x] get api-resources from kube cache
  - [ ] resource name completion
