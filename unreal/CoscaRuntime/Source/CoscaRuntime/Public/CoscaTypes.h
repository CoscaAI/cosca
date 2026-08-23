// CoscaTypes.h — shared data contract between Cosca (Go) and Unreal.
// Mirrors the JSON envelope from internal/bridge/client.go in Cosca.
#pragma once

#include "CoreMinimal.h"
#include "GameplayTagContainer.h"
#include "CoscaTypes.generated.h"

USTRUCT(BlueprintType)
struct FTransformPayload
{
	GENERATED_BODY()

	UPROPERTY(BlueprintReadOnly) FVector Position = FVector::ZeroVector;
	UPROPERTY(BlueprintReadOnly) FQuat Rotation = FQuat::Identity;
	UPROPERTY(BlueprintReadOnly) FVector Scale = FVector::OneVector;
};

// Message types (mirror bridge MessageType in Cosca)
UENUM(BlueprintType)
enum class ECoscaMessageType : uint8
{
	Frame        UMETA(DisplayName = "Frame"),
	Audio        UMETA(DisplayName = "Audio"),
	Event        UMETA(DisplayName = "Event"),
	StateSync    UMETA(DisplayName = "StateSync"),
	Action       UMETA(DisplayName = "Action"),
	Spawn        UMETA(DisplayName = "Spawn"),
	Destroy      UMETA(DisplayName = "Destroy"),
	Modify       UMETA(DisplayName = "Modify"),
	Weather      UMETA(DisplayName = "Weather"),
	Time         UMETA(DisplayName = "Time"),
	Ping         UMETA(DisplayName = "Ping"),
	Pong         UMETA(DisplayName = "Pong"),
	Error        UMETA(DisplayName = "Error"),
};

// Spawn command payload (Cosca -> Unreal)
USTRUCT(BlueprintType)
struct FSpawnPayload
{
	GENERATED_BODY()

	// Unique Cosca WorldEntity id (<= correlation id)
	UPROPERTY(BlueprintReadWrite) FString EntityId;
	// Semantic type: "npc", "object", "vehicle", etc.
	UPROPERTY(BlueprintReadWrite) FString Type;
	// Asset content hash (sha256) — used for dedup/provenance registration
	UPROPERTY(BlueprintReadWrite) FString AssetHash;
	// Position/Rotation/Scale in world space
	UPROPERTY(BlueprintReadWrite) FVector Position = FVector::ZeroVector;
	UPROPERTY(BlueprintReadWrite) FQuat Rotation = FQuat::Identity;
	UPROPERTY(BlueprintReadWrite) FVector Scale = FVector::OneVector;
	// Semantic GameplayTags (vocabulary, e.g. cosca.npc.guard)
	UPROPERTY(BlueprintReadWrite) FGameplayTagContainer Tags;
	// Arbitrary config (mesh, material, etc.)
	UPROPERTY(BlueprintReadWrite) TMap<FString, FString> Config;
};

// Action command payload (Cosca -> Unreal)
USTRUCT(BlueprintType)
struct FActionPayload
{
	GENERATED_BODY()

	UPROPERTY(BlueprintReadWrite) FString EntityId;
	// "move_to", "interact", "look_at", "speak"
	UPROPERTY(BlueprintReadWrite) FString Action;
	// action-specific params, e.g. {"target": [x,y,z]}
	UPROPERTY(BlueprintReadWrite) TMap<FString, FString> Params;
};

// Envelope for all Cosca<->Unreal messages.
USTRUCT(BlueprintType)
struct FCoscaMessage
{
	GENERATED_BODY()

	UPROPERTY(BlueprintReadWrite) ECoscaMessageType Type = ECoscaMessageType::Ping;
	UPROPERTY(BlueprintReadWrite) FString Id;
	UPROPERTY(BlueprintReadWrite) FString EntityId;
	// JSON payload string (serialized by Cosca, parsed server-side)
	UPROPERTY(BlueprintReadWrite) FString Payload;
};
