package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (in *GrafanaDashboard) matchesSelector(s *metav1.LabelSelector) (bool, error) {
	selector, err := metav1.LabelSelectorAsSelector(s)
	if err != nil {
		return false, err
	}

	return selector.Empty() || selector.Matches(labels.Set(in.Labels)), nil
}

// Check if the dashboard matches at least one of the selectors
func (in *GrafanaDashboard) MatchesSelectors(s []*metav1.LabelSelector) (bool, error) {
	result := false

	for _, selector := range s {
		match, err := in.matchesSelector(selector)
		if err != nil {
			return false, err
		}

		result = result || match
	}

	return result, nil
}

func (in *GrafanaDataSource) matchesSelector(s *metav1.LabelSelector) (bool, error) {
	selector, err := metav1.LabelSelectorAsSelector(s)
	if err != nil {
		return false, err
	}

	return selector.Empty() || selector.Matches(labels.Set(in.Labels)), nil
}

// Check if the dashboard matches at least one of the selectors
func (in *GrafanaDataSource) MatchesSelectors(s []*metav1.LabelSelector) (bool, error) {
	result := false

	for _, selector := range s {
		match, err := in.matchesSelector(selector)
		if err != nil {
			return false, err
		}

		result = result || match
	}

	return result, nil
}
