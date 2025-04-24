package vm

import (
	"fmt"

	"encoding/json"

	templatev1 "github.com/namu-north-america/templates/api/template.openshift.io/v1"

	v1 "kubevirt.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// VMFromTemplate returns the first VirtualMachine object from the template.
func VMFromTemplate(t templatev1.Template) (*v1.VirtualMachine, error) {
	if len(t.Objects) == 0 {
		return nil, fmt.Errorf("template has no objects")
	}

	var objMap map[string]interface{}
	if err := json.Unmarshal(t.Objects[0].Raw, &objMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw object: %w", err)
	}

	kind, ok := objMap["kind"].(string)
	if !ok || kind != "VirtualMachine" {
		return nil, fmt.Errorf("first object is not a VirtualMachine (kind=%v)", kind)
	}

	// Now that we've confirmed it's a VirtualMachine, decode into the typed struct.
	var machine v1.VirtualMachine
	if err := json.Unmarshal(t.Objects[0].Raw, &machine); err != nil {
		return nil, fmt.Errorf("failed to unmarshal into VirtualMachine: %w", err)
	}

	vm, err := ExtractDiskFromTemplate(t, &machine)
	if err != nil {
		return nil, fmt.Errorf("failed to extract disk from template: %w", err)
	}

	return vm, nil
}

func ExtractDiskFromTemplate(t templatev1.Template, vm *v1.VirtualMachine) (*v1.VirtualMachine, error) {
	if len(t.Objects) == 0 {
		return nil, fmt.Errorf("template has no objects")
	}

	// Iterate to find all DataVolume objects
	for _, rawObj := range t.Objects {
		var objMap map[string]interface{}
		if err := json.Unmarshal(rawObj.Raw, &objMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal object: %w", err)
		}

		kind, ok := objMap["kind"].(string)
		if !ok || kind != "DataVolume" {
			continue
		}

		var dataVolume cdiv1.DataVolume
		if err := json.Unmarshal(rawObj.Raw, &dataVolume); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into DataVolume: %w", err)
		}

		// Update matching DataVolumeTemplate
		found := false
		for i, dv := range vm.Spec.DataVolumeTemplates {
			if dv.Name == dataVolume.Name {
				vm.Spec.DataVolumeTemplates[i].Spec = dataVolume.Spec
				found = true
				break
			}
		}

		if !found {
			// Append new DataVolumeTemplate
			vm.Spec.DataVolumeTemplates = append(vm.Spec.DataVolumeTemplates, v1.DataVolumeTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: dataVolume.Name,
				},
				Spec: dataVolume.Spec,
			})
		}

		// Ensure a volume entry exists in the VM template
		volumes := vm.Spec.Template.Spec.Volumes
		diskName := dataVolume.Name

		volumeExists := false
		for _, vol := range volumes {
			if vol.Name == diskName {
				volumeExists = true
				break
			}
		}
		if !volumeExists {
			vm.Spec.Template.Spec.Volumes = append(volumes, v1.Volume{
				Name: diskName,
				VolumeSource: v1.VolumeSource{
					DataVolume: &v1.DataVolumeSource{
						Name: diskName,
					},
				},
			})
		}

		// Ensure a disk is defined in the domain devices
		disks := vm.Spec.Template.Spec.Domain.Devices.Disks
		diskExists := false
		for _, d := range disks {
			if d.Name == diskName {
				diskExists = true
				break
			}
		}
		if !diskExists {
			vm.Spec.Template.Spec.Domain.Devices.Disks = append(disks, v1.Disk{
				Name: diskName,
				DiskDevice: v1.DiskDevice{
					Disk: &v1.DiskTarget{
						Bus: "virtio",
					},
				},
			})
		}
	}

	return vm, nil
}
