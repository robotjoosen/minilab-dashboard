package discovery

import "testing"

func TestMatchesService(t *testing.T) {
	tests := []struct {
		name        string
		entryName   string
		serviceName string
		want        bool
	}{
		{
			name:        "matching agent instance",
			entryName:   "rocket._minilab-agent._tcp.local.",
			serviceName: "_minilab-agent._tcp",
			want:        true,
		},
		{
			name:        "unrelated service picked up from other LAN traffic",
			entryName:   "some-printer._ipp._tcp.local.",
			serviceName: "_minilab-agent._tcp",
			want:        false,
		},
		{
			name:        "esphome device when browsing for agents",
			entryName:   "esp-thing._esphomelib._tcp.local.",
			serviceName: "_minilab-agent._tcp",
			want:        false,
		},
		{
			name:        "agent instance when browsing for esphome devices",
			entryName:   "rocket._minilab-agent._tcp.local.",
			serviceName: "_esphomelib._tcp",
			want:        false,
		},
		{
			name:        "queried service name with a trailing dot still matches",
			entryName:   "rocket._minilab-agent._tcp.local.",
			serviceName: "_minilab-agent._tcp.",
			want:        true,
		},
		{
			name:        "substring match is not enough -- must be an exact service suffix",
			entryName:   "rocket._minilab-agent-test._tcp.local.",
			serviceName: "_minilab-agent._tcp",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesService(tt.entryName, tt.serviceName); got != tt.want {
				t.Errorf("matchesService(%q, %q) = %v, want %v", tt.entryName, tt.serviceName, got, tt.want)
			}
		})
	}
}
