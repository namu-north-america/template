package vm

import (
	"fmt"
	templatev1 "github/namu-north-america/templates/api/template.openshift.io/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	v1 "kubevirt.io/api/core/v1"
)

var (
	scheme = runtime.NewScheme()
	// register VirtualMachine to scheme
	_ = v1.AddToScheme(scheme)

	decoder = serializer.NewCodecFactory(scheme).UniversalDeserializer()
)

// VMFromTemplate extracts a VirtualMachine resource from an OpenShift-style template
func VMFromTemplate(template templatev1.Template) (*v1.VirtualMachine, error) {
	return nil, fmt.Errorf("no VirtualMachine found in template")
}
