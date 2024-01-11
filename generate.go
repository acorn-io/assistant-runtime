//go:generate go run github.com/acorn-io/baaah/cmd/deepcopy ./pkg/apis/assistant.acorn.io/v1/
//go:generate go run k8s.io/kube-openapi/cmd/openapi-gen -i github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io/v1,github.com/acorn-io/aml/pkg/jsonschema,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/runtime,k8s.io/apimachinery/pkg/version,k8s.io/apimachinery/pkg/api/resource,k8s.io/apimachinery/pkg/util/intstr -p ./pkg/openapi/generated -h tools/header.txt

package main
