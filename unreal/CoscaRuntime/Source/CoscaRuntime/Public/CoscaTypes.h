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

	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FVector Position = FVector::ZeroVector;
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FQuat Rotation = FQuat::Identity;
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FVector Scale = FVector::OneVector;
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
	ImportMesh   UMETA(DisplayName = "ImportMesh"),
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
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FString EntityId;
	// Semantic type: "npc", "object", "vehicle", etc.
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FString Type;
	// Asset content hash (sha256) — used for dedup/provenance registration
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FString AssetHash;
	// Position/Rotation/Scale in world space
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FVector Position = FVector::ZeroVector;
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FQuat Rotation = FQuat::Identity;
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FVector Scale = FVector::OneVector;
	// Semantic GameplayTags (vocabulary, e.g. cosca.npc.guard)
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FGameplayTagContainer Tags;
	// Arbitrary config (mesh, material, etc.)
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") TMap<FString, FString> Config;
};

// Action command payload (Cosca -> Unreal)
USTRUCT(BlueprintType)
struct FActionPayload
{
	GENERATED_BODY()

	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FString EntityId;
	// "move_to", "interact", "look_at", "speak"
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FString Action;
	// action-specific params, e.g. {"target": [x,y,z]}
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") TMap<FString, FString> Params;
};

// ImportMesh command payload (Cosca -> Unreal)
USTRUCT(BlueprintType)
struct FImportMeshPayload
{
	GENERATED_BODY()

	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FString EntityId;
	// AssetID or /Game/ content path
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FString MeshPath;
	// Semantic type (for future reference)
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FString Type;
	// Position/Rotation/Scale in world space
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FVector Position = FVector::ZeroVector;
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FQuat Rotation = FQuat::Identity;
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FVector Scale = FVector::OneVector;
};

// Time-of-day command payload (Cosca -> Unreal)
// Controls the day/night cycle: sun position, color, ambient, fog.
USTRUCT(BlueprintType)
struct FTimePayload
{
	GENERATED_BODY()

	// Hour of day, 0.0 (midnight) to 24.0 (next midnight). Fractional = minutes.
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") float Hour = 12.0f;
};

// Weather command payload (Cosca -> Unreal)
USTRUCT(BlueprintType)
struct FWeatherPayload
{
	GENERATED_BODY()

	// Weather type: "clear", "rain", "snow", "fog", "storm", "overcast"
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FString Type = TEXT("clear");
	// Intensity 0.0-1.0 (affects density of effects)
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") float Intensity = 1.0f;
};

// Envelope for all Cosca<->Unreal messages.
USTRUCT(BlueprintType)
struct FCoscaMessage
{
	GENERATED_BODY()

	UPROPERTY(BlueprintReadWrite, Category = "Cosca") ECoscaMessageType Type = ECoscaMessageType::Ping;
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FString Id;
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FString EntityId;
	// JSON payload string (serialized by Cosca, parsed server-side)
	UPROPERTY(BlueprintReadWrite, Category = "Cosca") FString Payload;
};
