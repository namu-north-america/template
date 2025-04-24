/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package templateopenshiftio

import (
	"context"
	"slices"

	templatev1 "github.com/namu-north-america/templates/api/template.openshift.io/v1"

	"github.com/namu-north-america/templates/pkg/vm"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	kubevirtclient "kubevirt.io/client-go/kubecli"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// custom finalizer
var TemplateFinalizer = "template.cocktail-virt.io/finalizer"

// TemplateReconciler reconciles a Template object
type TemplateReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	VirtClient kubevirtclient.KubevirtClient
}

// +kubebuilder:rbac:groups=template.openshift.io.templates.cocktail-virt.io,resources=templates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=template.openshift.io.templates.cocktail-virt.io,resources=templates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=template.openshift.io.templates.cocktail-virt.io,resources=templates/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Template object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile

func (r *TemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Template
	var tpl templatev1.Template
	if err := r.Get(ctx, req.NamespacedName, &tpl); err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, "unable to fetch Template")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	//  Honor the `provision` label
	if val, ok := tpl.Labels["provision"]; !ok || val != "true" {
		logger.Info("Provision label not set to true, skipping VM creation", "provision", tpl.Labels["provision"])
		return ctrl.Result{}, nil
	}

	// Produce a VirtualMachine object from the Template
	machine, err := vm.VMFromTemplate(tpl)
	if err != nil {
		logger.Error(err, "could not extract VM from template")
		meta.SetStatusCondition(&tpl.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "TemplateParseFailed",
			Message:            err.Error(),
			LastTransitionTime: metav1.Now(),
		})
		_ = r.Status().Update(ctx, &tpl)
		return ctrl.Result{}, nil
	}

	vmName := machine.Name

	machine.Namespace = tpl.Namespace
	if machine.Labels == nil {
		machine.Labels = map[string]string{}
	}
	machine.Labels["vm.kubevirt.io/template"] = tpl.Name
	machine.Labels["managed-by"] = "template-vm-controller"
	machine.Labels["vm.kubevirt.io/template-namespace"] = tpl.Namespace

	// if template is marked for deletion, remove the finalizer
	if tpl.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted
		if !slices.Contains(tpl.ObjectMeta.Finalizers, TemplateFinalizer) {
			// Add our finalizer for this CR
			tpl.ObjectMeta.Finalizers = append(tpl.ObjectMeta.Finalizers, TemplateFinalizer)
			if err := r.Update(ctx, &tpl); err != nil {
				logger.Error(err, "failed to update Template with finalizer")

				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if slices.Contains(tpl.ObjectMeta.Finalizers, TemplateFinalizer) {

			// delete vm
			vm, err := r.VirtClient.VirtualMachine(tpl.Namespace).Get(ctx, vmName, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					logger.Info("VM not found, ignoring", "vm", vmName)
					return ctrl.Result{}, nil
				} else {
					logger.Error(err, "failed to get VM", "vm", vmName)
					return ctrl.Result{}, err
				}
			}
			// delete the vm
			if err := r.VirtClient.VirtualMachine(tpl.Namespace).Delete(ctx, vm.Name, metav1.DeleteOptions{}); err != nil {
				if errors.IsNotFound(err) {
					logger.Info("VM not found, ignoring", "vm", vmName)
					return ctrl.Result{}, nil
				} else {
					logger.Error(err, "failed to delete VM", "vm", vmName)
					return ctrl.Result{}, err
				}
			}
			logger.Info("Deleted VM from template", "vm", vmName)

			// remove our finalizer from the list and update it.
			tpl.ObjectMeta.Finalizers = removeString(tpl.ObjectMeta.Finalizers, TemplateFinalizer)
			if err := r.Update(ctx, &tpl); err != nil {
				logger.Error(err, "failed to update Template with finalizer")

				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if _, err := r.VirtClient.VirtualMachine(tpl.Namespace).Get(ctx, vmName, metav1.GetOptions{}); err == nil {
		logger.Info("VM already exists, skipping", "vm", vmName)
		return ctrl.Result{}, nil
	} else if !errors.IsNotFound(err) {
		logger.Error(err, "error checking for existing VM", "vm", vmName)
		return ctrl.Result{}, err
	}

	// 6) Create the VM
	created, err := r.VirtClient.VirtualMachine(tpl.Namespace).Create(ctx, machine, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "failed to create VM", "vm", vmName)
		meta.SetStatusCondition(&tpl.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "VMCreationFailed",
			Message:            err.Error(),
			LastTransitionTime: metav1.Now(),
		})
		_ = r.Status().Update(ctx, &tpl)
		return ctrl.Result{}, err
	}

	logger.Info("Created VM from Template", "vm", created.Name)
	meta.SetStatusCondition(&tpl.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "VMCreated",
		Message:            "VM created successfully",
		LastTransitionTime: metav1.Now(),
	})
	_ = r.Status().Update(ctx, &tpl)
	return ctrl.Result{}, nil
}

func (r *TemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&templatev1.Template{}).
		Complete(r)
}

// removeString removes a string from a slice
func removeString(slice []string, str string) []string {
	for i, s := range slice {
		if s == str {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}
