// CoscaEntityComponent.cpp
#include "CoscaEntityComponent.h"
#include "GameFramework/Actor.h"

UCoscaEntityComponent::UCoscaEntityComponent()
{
	PrimaryComponentTick.bCanEverTick = false;
}

void UCoscaEntityComponent::InitFromSpawn(
	const FString& InEntityId,
	const FString& InType,
	const FString& InAssetHash,
	const FVector& InPosition,
	const FQuat& InRotation,
	const FVector& InScale,
	const FGameplayTagContainer& InTags)
{
	EntityId = InEntityId;
	AssetHash = InAssetHash;
	Tags = InTags;

	if (AActor* Owner = GetOwner())
	{
		Owner->SetActorLocationAndRotation(InPosition, InRotation, false, nullptr, ETeleportType::TeleportPhysics);
		Owner->SetActorScale3D(InScale);
	}
}
