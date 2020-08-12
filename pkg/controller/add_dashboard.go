package controller

import (
	"github.com/ucloud/grafana-operator/pkg/controller/grafanadashboard"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, grafanadashboard.Add)
}
