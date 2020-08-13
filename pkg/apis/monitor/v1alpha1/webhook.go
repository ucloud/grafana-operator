package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (in *GrafanaDashboard) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(in).
		Complete()
}

var _ webhook.Validator = &GrafanaDashboard{}

func (in *GrafanaDashboard) ValidateCreate() error {
	return nil
}

func (in *GrafanaDashboard) ValidateUpdate(old runtime.Object) error {
	oldObj, ok := old.(*GrafanaDashboard)
	if ok {
		if in.Hash() != oldObj.Hash() {
			return fmt.Errorf("GrafanaDashboard' Spec do not allowed changes")
		}
	}
	return nil
}

func (in *GrafanaDashboard) ValidateDelete() error {
	return nil
}

func (in *GrafanaDataSource) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(in).
		Complete()
}

var _ webhook.Validator = &GrafanaDataSource{}

func (in *GrafanaDataSource) ValidateCreate() error {
	return nil
}

func (in *GrafanaDataSource) ValidateUpdate(old runtime.Object) error {
	oldObj, ok := old.(*GrafanaDataSource)
	if ok {
		if in.Hash() != oldObj.Hash() {
			return fmt.Errorf("GrafanaDataSource' Spec do not allowed changes")
		}
	}
	return nil
}

func (in *GrafanaDataSource) ValidateDelete() error {
	return nil
}
