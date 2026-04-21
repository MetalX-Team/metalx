package app

import "testing"

func TestAlertLevel(t *testing.T) {
	testCases := []struct {
		name   string
		cpu    float64
		memory float64
		disk   float64
		want   string
	}{
		{name: "normal", cpu: 32, memory: 41, disk: 38, want: "normal"},
		{name: "warning", cpu: 75, memory: 40, disk: 38, want: "warning"},
		{name: "critical", cpu: 32, memory: 91, disk: 38, want: "critical"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := alertLevel(tc.cpu, tc.memory, tc.disk)
			if got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}
