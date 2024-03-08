# kubectl-download

Output and download any kubernetes resources in to named files.

It simplifies the process of `kubectl get pod nginx -oyaml > nginx.yaml`, you don't need pipe and name a file manually. Just `kubectl download pod nginx`, everything in "pods_my-pod.yaml".

## How to use

```sh
# kubectl krew install download
# PR is still pending, so use the following command to install

kubectl krew install --manifest-url https://raw.githubusercontent.com/krew-release-bot/krew-index/Anddd7-download-kubectl-download-v0.0.4/plugins/download.yaml

kubectl donwload pod
kubectl download deploy my-deploy
kubectl download ingress my-ingress -n my-namespace
```

### Completion

Register completion for plugins for kubectl

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
- [x] support krew install
- [ ] filter server fields (e.g. managed fields)
  - drop specific fields in yaml file
  - or delete key from map interface before marshalling
- [x] github workflow
- [x] renovate
- [x] zsh complete
  - [ ] get api-resources from kube cache
  - [ ] resource name completion