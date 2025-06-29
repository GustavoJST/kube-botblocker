#!/usr/bin/env bash

cat <<EOF > test-env.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: test-env
EOF

if [[ -z $1 ]]; then
  echo "[ERROR] Missing number of test workloads to create"
  exit 1
fi

for i in $(seq 1 $1); do
cat <<EOF >> test-env.yaml

---

apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-env-$i
  namespace: test-env
  labels:
    testing: "true"
    name: "test-env-$i"
spec:
  selector:
    matchLabels:
      app: test-env-$i
  template:
    metadata:
      labels:
        app: test-env-$i
    spec:
      containers:
      - name: test-env-$i
        image: gcr.io/google-samples/env-show:1.1
        ports:
        - containerPort: 8080
          name: http
        env:
          - name: POD_NAME
            valueFrom:
              fieldRef:
                fieldPath: metadata.name
          - name: POD_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          - name: APP_NAME
            valueFrom:
              fieldRef:
                fieldPath: metadata.labels['app']
          - name: USER_VAR
            value: "Hello from workload \$(APP_NAME)"
---
apiVersion: v1
kind: Service
metadata:
  name: test-env-$i
  namespace: test-env
  labels:
    testing: "true"
    name: test-env-$i
spec:
  type: NodePort
  selector:
    app: test-env-$i
  ports:
  - port: 8080
    targetPort: 8080
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-env-$i
  namespace: test-env
  labels:
    testing: "true"
    name: test-env-$i
spec:
  rules:
  - host: test-env-$i.example
    http:
      paths:
      - pathType: Prefix
        path: "/"
        backend:
          service:
            name: test-env-$i
            port:
              number: 8080
EOF
done