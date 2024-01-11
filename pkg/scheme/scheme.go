package scheme

import (
	v1 "github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io/v1"
	"github.com/acorn-io/baaah/pkg/restconfig"
	acornv1 "github.com/acorn-io/runtime/pkg/apis/api.acorn.io/v1"
	acorninternalv1 "github.com/acorn-io/runtime/pkg/apis/internal.acorn.io/v1"
	corev1 "k8s.io/api/core/v1"
)

var (
	Scheme,
	Codecs,
	Parameter,
	AddToScheme = restconfig.MustBuildScheme(
		acornv1.AddToScheme,
		acorninternalv1.AddToScheme,
		v1.AddToScheme,
		corev1.AddToScheme)
)
