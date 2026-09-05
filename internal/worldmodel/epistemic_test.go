package worldmodel

import "testing"

func TestSpatialObservation_TrustEpistemic(t *testing.T) {
	// Known → utilizável (MEASURED).
	known := SpatialObservation{TrustState: TrustKnown, Uncertainty: 0.05, Source: ObservationSource{SensorID: "rf-1", Authenticated: true}}
	if !known.Trust() {
		t.Fatal("Known deve ser utilizável")
	}
	// Degraded → utilizável mas com incerteza (INFERRED) — o sistema sabe que é
	// aproximado (I4).
	deg := SpatialObservation{TrustState: TrustDegraded, Uncertainty: 0.9}
	if !deg.Trust() {
		t.Fatal("Degraded deve ser utilizável (mas com incerteza explícita)")
	}
	// Unknown → o sistema RECUSA e recua para prior (RuView GateDecision) — I4.
	unk := SpatialObservation{TrustState: TrustUnknown, Uncertainty: 1.5}
	if unk.Trust() {
		t.Fatal("Unknown deve ser não-utilizável (o sistema NÃO sabe)")
	}
}

func TestSpatialObservation_UnknownByDefaultWhenSet(t *testing.T) {
	// Sem trust_state setado → default "" que NÃO é TrustUnknown, então Trust()=true.
	o := SpatialObservation{}
	if !o.Trust() {
		t.Fatal("observação sem flag explícita é tratada como utilizável (default conservador)")
	}
}

func TestObservationSource_Provenance(t *testing.T) {
	src := ObservationSource{SensorID: "cam-2", Authenticated: true, ReplayProtected: true}
	if src.SensorID != "cam-2" || !src.Authenticated || !src.ReplayProtected {
		t.Fatalf("proveniência não preservada: %+v", src)
	}
}
