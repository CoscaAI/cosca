package grpcserver

import (
	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"github.com/CoscaAI/cosca/internal/runtime"
)

// runtimeStatusToPb converts a runtime.Runtime snapshot into a protobuf
// StatusResponse. It extracts state, health, uptime, version, and
// component information from the runtime's state snapshot.
func runtimeStatusToPb(rt *runtime.Runtime) *cospb.StatusResponse {
	if rt == nil {
		return &cospb.StatusResponse{}
	}
	state := rt.State().Get()
	cfg := rt.Config()

	components := make(map[string]*cospb.ComponentInfo, len(state.Components))
	for name, info := range state.Components {
		components[name] = &cospb.ComponentInfo{
			Name:          info.Name,
			Status:        string(info.Status),
			UptimeSeconds: info.Uptime.Seconds(),
		}
	}

	return &cospb.StatusResponse{
		State:         state.CurrentState.String(),
		Health:        string(state.Health),
		UptimeSeconds: state.Uptime.Seconds(),
		Version:       cfg.Version,
		Components:    components,
	}
}

// runtimeHealthToPb converts a runtime.Runtime health check into a
// protobuf HealthResponse. It reports healthy when the runtime status is
// healthy or unknown, and collects warnings from degraded components.
func runtimeHealthToPb(rt *runtime.Runtime) *cospb.HealthResponse {
	if rt == nil {
		return &cospb.HealthResponse{}
	}
	health := rt.Health()
	warnings := make([]string, 0)

	if health == runtime.StatusDegraded {
		for name, info := range rt.State().AllComponentStatuses() {
			if info.Status == runtime.StatusDegraded {
				warnings = append(warnings, name+": "+info.Message)
			}
		}
	}

	healthy := health == runtime.StatusHealthy || health == runtime.StatusUnknown

	return &cospb.HealthResponse{
		Healthy:  healthy,
		Warnings: warnings,
	}
}
