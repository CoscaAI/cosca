package contracts

import (
	"strings"
	"testing"
)

// ---- Fixtures ---- //

type agentCreateReqV1 struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

type agentCreateRespV1 struct {
	ID string `json:"id"`
}

// V1.1 adds an optional field — additive, valid minor.
type agentCreateReqV11 struct {
	Name    string  `json:"name"`
	Model   string  `json:"model"`
	Profile *string `json:"profile,omitempty"` // new optional field
}

// V1.2 changes an existing field type AND keeps the field added in V1.1 —
// the ONLY violation vs V1.1 must be the Model type change, so the test is
// deterministic (the validator walks fields in map order).
type agentCreateReqV12 struct {
	Name    string  `json:"name"`
	Model   int     `json:"model"`             // changed type (the one violation)
	Profile *string `json:"profile,omitempty"` // kept from V1.1
}

// V2.0 removes a field — breaking change, valid major.
type agentCreateReqV2 struct {
	Name string `json:"name"`
}

func noopRequest(_ any) (any, error) { return nil, nil }

// ---- Invariant 1: structural integrity ---- //

func TestValidateRegistry_EmptyRegistryOK(t *testing.T) {
	r := NewRegistry()
	if err := ValidateRegistry(r, nil); err != nil {
		t.Fatalf("empty registry must validate: %v", err)
	}
}

func TestValidateRegistry_ValidSingleContract(t *testing.T) {
	r := NewRegistry()
	r.Methods["agent.create"] = MethodRegistry{MajorLines: map[int]MajorLine{
		1: {LatestMinor: 0, Versions: map[int]VersionEntry{
			0: {Contract: RpcContract{
				Method: "agent.create", SchemaVersion: SchemaVersion{1, 0},
				Request: agentCreateReqV1{}, Response: agentCreateRespV1{},
			}},
		}},
	}}
	if err := ValidateRegistry(r, []string{"agent.create"}); err != nil {
		t.Fatalf("valid registry must pass: %v", err)
	}
}

func TestValidateRegistry_LatestMinorMustBeInstalled(t *testing.T) {
	r := NewRegistry()
	r.Methods["agent.create"] = MethodRegistry{MajorLines: map[int]MajorLine{
		1: {LatestMinor: 2, Versions: map[int]VersionEntry{ // latestMinor 2 not installed
			0: {Contract: RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{1, 0}, Request: agentCreateReqV1{}, Response: agentCreateRespV1{}}},
		}},
	}}
	err := ValidateRegistry(r, nil)
	if err == nil || !strings.Contains(err.Error(), "latestMinor 2 is not installed") {
		t.Fatalf("expected latestMinor-not-installed error, got: %v", err)
	}
}

func TestValidateRegistry_LatestMinorMustBeHighestInstalled(t *testing.T) {
	r := NewRegistry()
	line := MajorLine{
		LatestMinor: 0,
		Versions: map[int]VersionEntry{
			0: {Contract: RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{1, 0}, Request: agentCreateReqV1{}, Response: agentCreateRespV1{}}},
			1: {Contract: RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{1, 1}, Request: agentCreateReqV1{}, Response: agentCreateRespV1{}}},
		},
	}
	r.Methods["agent.create"] = MethodRegistry{MajorLines: map[int]MajorLine{1: line}}
	err := ValidateRegistry(r, nil)
	if err == nil || !strings.Contains(err.Error(), "must be the highest installed minor") {
		t.Fatalf("expected highest-installed-minor error, got: %v", err)
	}
}

func TestValidateRegistry_FirstVersionNoUpgradePath(t *testing.T) {
	r := NewRegistry()
	r.Methods["m"] = MethodRegistry{MajorLines: map[int]MajorLine{
		1: {LatestMinor: 0, Versions: map[int]VersionEntry{
			0: {
				Contract:            RpcContract{Method: "m", SchemaVersion: SchemaVersion{1, 0}},
				UpgradeFromPrevious: &UpgradePath{From: SchemaVersion{1, 0}, To: SchemaVersion{1, 0}, UpgradeRequest: noopRequest, UpgradeResponse: noopRequest},
			},
		}},
	}}
	err := ValidateRegistry(r, nil)
	if err == nil || !strings.Contains(err.Error(), "without a previous installed version") {
		t.Fatalf("expected no-previous-version error, got: %v", err)
	}
}

func TestValidateRegistry_UpgradePathMustChain(t *testing.T) {
	r := NewRegistry()
	r.Methods["m"] = MethodRegistry{MajorLines: map[int]MajorLine{
		1: {LatestMinor: 1, Versions: map[int]VersionEntry{
			0: {Contract: RpcContract{Method: "m", SchemaVersion: SchemaVersion{1, 0}}},
			1: {
				Contract:            RpcContract{Method: "m", SchemaVersion: SchemaVersion{1, 1}},
				UpgradeFromPrevious: &UpgradePath{From: SchemaVersion{1, 0}, To: SchemaVersion{1, 1}}, // no adapters
			},
		}},
	}}
	err := ValidateRegistry(r, nil)
	if err == nil || !strings.Contains(err.Error(), "must declare UpgradeRequest") {
		t.Fatalf("expected missing-adapter error, got: %v", err)
	}
}

// ---- Invariant 2: minor is additive ---- //

func TestValidateRegistry_ValidMinorAdditive(t *testing.T) {
	r := NewRegistry()
	r.Methods["agent.create"] = MethodRegistry{MajorLines: map[int]MajorLine{
		1: {LatestMinor: 1, Versions: map[int]VersionEntry{
			0: {Contract: RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{1, 0}, Request: agentCreateReqV1{}, Response: agentCreateRespV1{}}},
			1: {
				Contract:            RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{1, 1}, Request: agentCreateReqV11{}, Response: agentCreateRespV1{}},
				UpgradeFromPrevious: &UpgradePath{From: SchemaVersion{1, 0}, To: SchemaVersion{1, 1}, UpgradeRequest: noopRequest, UpgradeResponse: noopRequest},
			},
		}},
	}}
	if err := ValidateRegistry(r, []string{"agent.create"}); err != nil {
		t.Fatalf("additive minor must pass: %v", err)
	}
}

func TestValidateRegistry_InvalidMinorChangedFieldType(t *testing.T) {
	r := NewRegistry()
	r.Methods["agent.create"] = MethodRegistry{MajorLines: map[int]MajorLine{
		1: {LatestMinor: 2, Versions: map[int]VersionEntry{
			0: {Contract: RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{1, 0}, Request: agentCreateReqV1{}, Response: agentCreateRespV1{}}},
			1: {
				Contract:            RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{1, 1}, Request: agentCreateReqV11{}, Response: agentCreateRespV1{}},
				UpgradeFromPrevious: &UpgradePath{From: SchemaVersion{1, 0}, To: SchemaVersion{1, 1}, UpgradeRequest: noopRequest, UpgradeResponse: noopRequest},
			},
			2: {
				Contract:            RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{1, 2}, Request: agentCreateReqV12{}, Response: agentCreateRespV1{}},
				UpgradeFromPrevious: &UpgradePath{From: SchemaVersion{1, 1}, To: SchemaVersion{1, 2}, UpgradeRequest: noopRequest, UpgradeResponse: noopRequest},
			},
		}},
	}}
	err := ValidateRegistry(r, nil)
	if err == nil || !strings.Contains(err.Error(), `changed field "Model"`) {
		t.Fatalf("expected field-type-change error, got: %v", err)
	}
}

func TestValidateRegistry_InvalidMinorRemovedField(t *testing.T) {
	r := NewRegistry()
	r.Methods["agent.create"] = MethodRegistry{MajorLines: map[int]MajorLine{
		1: {LatestMinor: 1, Versions: map[int]VersionEntry{
			0: {Contract: RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{1, 0}, Request: agentCreateReqV1{}, Response: agentCreateRespV1{}}},
			1: {
				Contract:            RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{1, 1}, Request: agentCreateReqV2{}, Response: agentCreateRespV1{}},
				UpgradeFromPrevious: &UpgradePath{From: SchemaVersion{1, 0}, To: SchemaVersion{1, 1}, UpgradeRequest: noopRequest, UpgradeResponse: noopRequest},
			},
		}},
	}}
	err := ValidateRegistry(r, nil)
	if err == nil || !strings.Contains(err.Error(), `removed field "Model"`) {
		t.Fatalf("expected removed-field error, got: %v", err)
	}
}

// ---- Invariant 3: major is breaking ---- //

func TestValidateRegistry_ValidMajorBreaking(t *testing.T) {
	r := NewRegistry()
	r.Methods["agent.create"] = MethodRegistry{MajorLines: map[int]MajorLine{
		1: {LatestMinor: 0, Versions: map[int]VersionEntry{
			0: {Contract: RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{1, 0}, Request: agentCreateReqV1{}, Response: agentCreateRespV1{}}},
		}},
		2: {LatestMinor: 0, Versions: map[int]VersionEntry{
			0: {
				Contract:            RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{2, 0}, Request: agentCreateReqV2{}, Response: agentCreateRespV1{}},
				UpgradeFromPrevious: &UpgradePath{From: SchemaVersion{1, 0}, To: SchemaVersion{2, 0}, UpgradeRequest: noopRequest, UpgradeResponse: noopRequest},
			},
		}, DowngradePathsFromLatest: map[int]DowngradePath{
			1: {From: SchemaVersion{2, 0}, To: SchemaVersion{1, 0}, DowngradeRequest: noopRequest, DowngradeResponse: noopRequest},
		}},
	}}
	if err := ValidateRegistry(r, []string{"agent.create"}); err != nil {
		t.Fatalf("breaking major must pass: %v", err)
	}
}

func TestValidateRegistry_InvalidMajorNotBreaking(t *testing.T) {
	r := NewRegistry()
	r.Methods["agent.create"] = MethodRegistry{MajorLines: map[int]MajorLine{
		1: {LatestMinor: 0, Versions: map[int]VersionEntry{
			0: {Contract: RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{1, 0}, Request: agentCreateReqV1{}, Response: agentCreateRespV1{}}},
		}},
		2: {LatestMinor: 0, Versions: map[int]VersionEntry{
			0: {
				Contract:            RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{2, 0}, Request: agentCreateReqV1{}, Response: agentCreateRespV1{}}, // same as v1!
				UpgradeFromPrevious: &UpgradePath{From: SchemaVersion{1, 0}, To: SchemaVersion{2, 0}, UpgradeRequest: noopRequest, UpgradeResponse: noopRequest},
			},
		}},
	}}
	err := ValidateRegistry(r, nil)
	if err == nil || !strings.Contains(err.Error(), "could have shipped as a minor") {
		t.Fatalf("expected not-breaking error, got: %v", err)
	}
}

// ---- Invariant 4: downgrades and degrades ---- //

func TestValidateRegistry_InvalidDowngradeTargetsSameOrNewerMajor(t *testing.T) {
	r := NewRegistry()
	r.Methods["m"] = MethodRegistry{MajorLines: map[int]MajorLine{
		1: {LatestMinor: 0, Versions: map[int]VersionEntry{
			0: {Contract: RpcContract{Method: "m", SchemaVersion: SchemaVersion{1, 0}}},
		}},
		2: {LatestMinor: 0, Versions: map[int]VersionEntry{
			0: {
				Contract:            RpcContract{Method: "m", SchemaVersion: SchemaVersion{2, 0}},
				UpgradeFromPrevious: &UpgradePath{From: SchemaVersion{1, 0}, To: SchemaVersion{2, 0}, UpgradeRequest: noopRequest, UpgradeResponse: noopRequest},
			},
		}, DowngradePathsFromLatest: map[int]DowngradePath{
			2: {From: SchemaVersion{2, 0}, To: SchemaVersion{2, 0}, DowngradeRequest: noopRequest, DowngradeResponse: noopRequest}, // targets same major
		}},
	}}
	err := ValidateRegistry(r, nil)
	if err == nil || !strings.Contains(err.Error(), "must target an older major") {
		t.Fatalf("expected older-major error, got: %v", err)
	}
}

func TestValidateRegistry_NonFloorMethodNeedsDegrade(t *testing.T) {
	r := NewRegistry()
	r.Methods["agent.create"] = MethodRegistry{MajorLines: map[int]MajorLine{
		1: {LatestMinor: 0, Versions: map[int]VersionEntry{
			0: {Contract: RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{1, 0}, Request: agentCreateReqV1{}, Response: agentCreateRespV1{}}},
		}},
	}}
	// agent.create NOT in floor list → must declare degrade.
	err := ValidateRegistry(r, []string{"session.init"})
	if err == nil || !strings.Contains(err.Error(), "must declare a degrade strategy") {
		t.Fatalf("expected degrade-required error, got: %v", err)
	}
}

func TestValidateRegistry_FloorMethodNoDegradeNeeded(t *testing.T) {
	r := NewRegistry()
	r.Methods["session.init"] = MethodRegistry{MajorLines: map[int]MajorLine{
		1: {LatestMinor: 0, Versions: map[int]VersionEntry{
			0: {Contract: RpcContract{Method: "session.init", SchemaVersion: SchemaVersion{1, 0}}},
		}},
	}}
	if err := ValidateRegistry(r, []string{"session.init"}); err != nil {
		t.Fatalf("floor method must validate without degrade: %v", err)
	}
}

func TestValidateRegistry_UnsupportedDegradeValid(t *testing.T) {
	r := NewRegistry()
	r.Methods["agent.create"] = MethodRegistry{
		MajorLines: map[int]MajorLine{
			1: {LatestMinor: 0, Versions: map[int]VersionEntry{
				0: {Contract: RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{1, 0}}},
			}},
		},
		Degrade: &Degrade{Strategy: DegradeUnsupported},
	}
	if err := ValidateRegistry(r, []string{"session.init"}); err != nil {
		t.Fatalf("unsupported degrade must validate: %v", err)
	}
}

func TestValidateRegistry_FallbackDegradeMustTargetFloor(t *testing.T) {
	r := NewRegistry()
	r.Methods["agent.fancy"] = MethodRegistry{
		MajorLines: map[int]MajorLine{
			1: {LatestMinor: 0, Versions: map[int]VersionEntry{
				0: {Contract: RpcContract{Method: "agent.fancy", SchemaVersion: SchemaVersion{1, 0}}},
			}},
		},
		Degrade: &Degrade{Strategy: DegradeFallback, Fallback: &FallbackDegrade{
			ToMethod: "session.init", // NOT a floor method
		}},
	}
	err := ValidateRegistry(r, []string{"agent.create"})
	if err == nil || !strings.Contains(err.Error(), "must target a floor method") {
		t.Fatalf("expected floor-target error, got: %v", err)
	}
}

func TestValidateRegistry_FallbackDegradeValid(t *testing.T) {
	r := NewRegistry()
	r.Methods["agent.create"] = MethodRegistry{MajorLines: map[int]MajorLine{
		1: {LatestMinor: 0, Versions: map[int]VersionEntry{
			0: {Contract: RpcContract{Method: "agent.create", SchemaVersion: SchemaVersion{1, 0}}},
		}},
	}}
	r.Methods["agent.fancy"] = MethodRegistry{
		MajorLines: map[int]MajorLine{
			1: {LatestMinor: 0, Versions: map[int]VersionEntry{
				0: {Contract: RpcContract{Method: "agent.fancy", SchemaVersion: SchemaVersion{1, 0}}},
			}},
		},
		Degrade: &Degrade{Strategy: DegradeFallback, Fallback: &FallbackDegrade{
			ToMethod:      "agent.create",
			ToVersion:     SchemaVersion{1, 0},
			AdaptRequest:  noopRequest,
			AdaptResponse: noopRequest,
		}},
	}
	if err := ValidateRegistry(r, []string{"agent.create"}); err != nil {
		t.Fatalf("valid fallback degrade must pass: %v", err)
	}
}

// ---- helpers ---- //

func TestPayloadEquivalent_StructFieldChange(t *testing.T) {
	if payloadEquivalent(agentCreateReqV1{}, agentCreateReqV2{}) {
		t.Fatal("structs with different fields must not be equivalent")
	}
	if !payloadEquivalent(agentCreateReqV1{}, agentCreateReqV1{}) {
		t.Fatal("same struct must be equivalent")
	}
	if !payloadEquivalent(nil, nil) {
		t.Fatal("nil and nil must be equivalent")
	}
	if payloadEquivalent(nil, agentCreateReqV1{}) {
		t.Fatal("nil and non-nil must not be equivalent")
	}
}
