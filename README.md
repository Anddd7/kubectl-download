# kubectl-download

Output and download any kubernetes resources in to named files.

It simplifies the process of `kubectl get pod nginx -oyaml > nginx.yaml`, you don't need pipe and name a file manually. Just `kubectl download pod nginx`, everything in "pods_my-pod.yaml".

## How to use

```sh
kubectl krew install download

kubectl donwload pod
kubectl download deploy my-deploy
kubectl download ingress my-ingress -n my-namespace
```

## TODO

- [x] docs
- [x] support krew install
- [ ] filter server fields (e.g. managed fields)
- [x] github workflow
- [x] renovate
- [ ] zsh complete
