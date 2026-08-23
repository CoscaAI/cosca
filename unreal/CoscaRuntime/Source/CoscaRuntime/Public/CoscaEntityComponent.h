// CoscaEntityComponent.h — binds a Cosca WorldEntity id to an AActor.
#pragma once

#include "CoreMinimal.h"
#include "Components/ActorComponent.h"
#include "GameplayTagContainer.h"
#include "CoscaEntityComponent.generated.h"

UCLASS(ClassGroup = (Cosca), meta = (BlueprintSpawnableComponent))
class COSCARUNTIME_API UCoscaEntityComponent : public UActorComponent
{
	GENERATED_BODY()

public:
	UCoscaEntityComponent();

	// Cosca WorldEntity id (correlation key)
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "Cosca")
	FString EntityId;

	// Asset content hash (sha256) for provenance/dedup
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "Cosca")
	FString AssetHash;

	// Semantic tags (vocabulary)
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "Cosca")
	FGameplayTagContainer Tags;

	// Initialize from Cosca spawn payload
	void InitFromSpawn(
		const FString& InEntityId,
		const FString& InType,
		const FString& InAssetHash,
		const FVector& InPosition,
		const FQuat& InRotation,
		const FVector& InScale,
		const FGameplayTagContainer& InTags);
};
