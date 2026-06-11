package events

import (
	"testing"

	chaosTypes "github.com/litmuschaos/chaos-operator/api/litmuschaos/v1alpha1"
)

func TestMapStatus(t *testing.T) {
	tests := []struct {
		in   chaosTypes.EngineStatus
		want string
	}{
		{chaosTypes.EngineStatusInitialized, "Running"},
		{chaosTypes.EngineStatusCompleted, "Succeeded"},
		{chaosTypes.EngineStatusStopped, "Stopped"},
		{chaosTypes.EngineStatus("anything-else"), "Running"},
		{chaosTypes.EngineStatus(""), "Running"},
	}
	for _, tt := range tests {
		t.Run(string(tt.in), func(t *testing.T) {
			if got := mapStatus(tt.in); got != tt.want {
				t.Errorf("mapStatus(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
