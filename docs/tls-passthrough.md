# TLS Passthrough Support (alpha)

[Kubernetes Gateway API](https://gateway-api.sigs.k8s.io/guides/tls/) lays out the general guidelines on how to configure TLS passthrough. Here are examples on how to use them against AWS VPC Lattice.

## Upgrade Controller

### Install gateway API TLSRoute CRD

Here is [TLSRoute CRD](https://github.com/liwenwu-amazon/aws-application-networking-k8-publics/blob/tls-route-support/config/crds/bases/gateway.networking.k8s.io_tlsroutes.yaml).



```
# clone the TLS support alpha repo
git clone git@github.com:liwenwu-amazon/aws-application-networking-k8-publics.git

# install CRD
kubectl apply -f config/crds/bases/gateway.networking.k8s.io_tlsroutes.yaml

# Verfiy TLSRoute CRD 
kubectl get crd tlsroutes.gateway.networking.k8s.io 
NAME                                  CREATED AT
tlsroutes.gateway.networking.k8s.io   2024-03-07T23:16:22Z

```

### Upgrade Controller Image

```
kubectl apply -f deploy-v1.0.3-tls.yaml
```

## Setup

### 