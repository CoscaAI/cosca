// CoscaWorldSubsystem.h — the Cosca hub. Runs a WebSocket server, receives
// commands from the Cosca orchestrator (Go), and executes them in the world.
#pragma once

#include "CoreMinimal.h"
#include "Subsystems/WorldSubsystem.h"
#include "Containers/Map.h"
#include "CoscaTypes.h"
#include "CoscaWorldSubsystem.generated.h"

class INetworkingWebSocket;
class IWebSocketServer;
class FJsonObject;

UCLASS()
class COSCARUNTIME_API UCoscaWorldSubsystem : public UTickableWorldSubsystem
{
	GENERATED_BODY()

public:
	UCoscaWorldSubsystem();
	virtual ~UCoscaWorldSubsystem();

	// UWorldSubsystem interface
	virtual void Initialize(FSubsystemCollectionBase& Collection) override;
	virtual void Deinitialize() override;
	virtual void OnWorldBeginPlay(UWorld& InWorld) override;

	// ---- Server lifecycle ----
	// Start the WebSocket server (call from config or GameMode).
	UFUNCTION(BlueprintCallable, Category = "Cosca")
	bool StartServer(int32 Port = 9000, const FString& BindAddress = TEXT(""));

	UFUNCTION(BlueprintCallable, Category = "Cosca")
	void StopServer();

	// ---- Spawning ----
	// Spawn an Actor for a Cosca entity (from a spawn command).
	UFUNCTION(BlueprintCallable, Category = "Cosca")
	AActor* SpawnEntity(const FSpawnPayload& Payload);

	// Remove an Actor by Cosca entity id.
	UFUNCTION(BlueprintCallable, Category = "Cosca")
	bool DestroyEntity(const FString& EntityId);

	// Import a GLB/FBX mesh and spawn it in the world.
	UFUNCTION(BlueprintCallable, Category = "Cosca")
	AActor* ImportMesh(const FImportMeshPayload& Payload);

	// Move an Actor to a target (uses NavMesh pathfinding).
	UFUNCTION(BlueprintCallable, Category = "Cosca")
	bool MoveEntity(const FString& EntityId, const FVector& Target);

	// ---- Broadcasting ----
	// Send a message back to the Cosca client.
	void SendToCosca(const FCoscaMessage& Message);

	// Find an Actor by Cosca entity id.
	AActor* FindEntity(const FString& EntityId) const;

	// Resolve an AssetID to a UE content path via the asset registry.
	FString ResolveAssetID(const FString& AssetId);

	// ---- Tick ----
	virtual void Tick(float DeltaTime) override;
	virtual TStatId GetStatId() const override;

private:
	// WebSocket server (TSharedPtr avoids needing the complete IWebSocketServer
	// type in the UHT-generated destructor) + active connection.
	TSharedPtr<IWebSocketServer> Server;
	INetworkingWebSocket* ClientSocket = nullptr;

	// entity id -> Actor
	TMap<FString, TWeakObjectPtr<AActor>> EntityMap;

	// Message handlers
	void OnClientConnected(INetworkingWebSocket* Socket);
	void OnPacketReceived(void* Data, int32 DataSize);
	void OnSocketClosed();
	void HandleCommand(const FCoscaMessage& Message);
	void HandleSpawn(const FSpawnPayload& Payload);
	void HandleAction(const FActionPayload& Payload);
	void HandleDestroy(const FString& EntityId);

	// Tick the WS server (libwebsocket typically requires a tick)
	void TickServer();

	// Parse a JSON payload into a struct
	bool ParseSpawn(const FString& Json, FSpawnPayload& Out);
	bool ParseAction(const FString& Json, FActionPayload& Out);
	bool ParseImportMesh(const FString& Json, FImportMeshPayload& Out);

	// JSON envelope helpers
	FCoscaMessage MessageFromJson(const TSharedPtr<FJsonObject>& Obj);
	FString MessageToJson(const FCoscaMessage& Message);

	// Type <-> string mapping
	ECoscaMessageType TypeFromString(const FString& S);
	FString TypeToString(ECoscaMessageType T);
};
