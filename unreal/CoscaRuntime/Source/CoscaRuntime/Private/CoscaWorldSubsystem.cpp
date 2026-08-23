// CoscaWorldSubsystem.cpp
#include "CoscaWorldSubsystem.h"
#include "CoscaEntityComponent.h"
#include "Engine/World.h"
#include "Engine/Engine.h"
#include "Engine/StaticMeshActor.h"
#include "Materials/MaterialInstanceDynamic.h"
#include "GameFramework/Character.h"
#include "GameFramework/Actor.h"
#include "GameFramework/PlayerController.h"
#include "NavigationSystem.h"
#include "NavigationPath.h"
#include "Modules/ModuleManager.h"
#include "IWebSocketNetworkingModule.h"
#include "IWebSocketServer.h"
#include "INetworkingWebSocket.h"
#include "Serialization/JsonSerializer.h"
#include "Serialization/JsonWriter.h"

DEFINE_LOG_CATEGORY_STATIC(LogCosca, Log, All);

UCoscaWorldSubsystem::UCoscaWorldSubsystem()
{
}

UCoscaWorldSubsystem::~UCoscaWorldSubsystem()
{
	// Defined here where IWebSocketServer is a complete type so the
	// TUniquePtr<IWebSocketServer> member destructor can be generated.
}

void UCoscaWorldSubsystem::Initialize(FSubsystemCollectionBase& Collection)
{
	Super::Initialize(Collection);
	UE_LOG(LogCosca, Log, TEXT("[Cosca] WorldSubsystem initialized."));
}

void UCoscaWorldSubsystem::Deinitialize()
{
	StopServer();
	Super::Deinitialize();
}

void UCoscaWorldSubsystem::OnWorldBeginPlay(UWorld& InWorld)
{
	Super::OnWorldBeginPlay(InWorld);
	StartServer(9000, TEXT(""));
}

// ---- Server ----

bool UCoscaWorldSubsystem::StartServer(int32 Port, const FString& BindAddress)
{
	if (!FModuleManager::Get().ModuleExists(TEXT("WebSocketNetworking")))
	{
		UE_LOG(LogCosca, Error, TEXT("[Cosca] WebSocketNetworking module not present. Enable the plugin."));
		return false;
	}

	IWebSocketNetworkingModule& WsModule = FModuleManager::LoadModuleChecked<IWebSocketNetworkingModule>(TEXT("WebSocketNetworking"));
	// CreateServer() returns TUniquePtr<IWebSocketServer>; wrap into TSharedPtr.
	Server = TSharedPtr<IWebSocketServer>(WsModule.CreateServer().Release());
	if (!Server.IsValid())
	{
		UE_LOG(LogCosca, Error, TEXT("[Cosca] Failed to create WebSocket server."));
		return false;
	}

	bool bOk = Server->Init(Port, FWebSocketClientConnectedCallBack::CreateUObject(this, &UCoscaWorldSubsystem::OnClientConnected), BindAddress);
	if (bOk)
	{
		UE_LOG(LogCosca, Log, TEXT("[Cosca] WebSocket server listening on port %d"), Port);
	}
	else
	{
		UE_LOG(LogCosca, Error, TEXT("[Cosca] WebSocket server failed to bind port %d"), Port);
	}
	return bOk;
}

void UCoscaWorldSubsystem::StopServer()
{
	if (Server.IsValid())
	{
		Server.Reset();
	}
	ClientSocket = nullptr;
}

void UCoscaWorldSubsystem::Tick(float DeltaTime)
{
	// UTickableWorldSubsystem has no Super::Tick (it's an FTickableGameObject).
	// Just service the WebSocket server and active connection.
	TickServer();
}

TStatId UCoscaWorldSubsystem::GetStatId() const
{
	RETURN_QUICK_DECLARE_CYCLE_STAT(UCoscaWorldSubsystem, STATGROUP_Tickables);
}

void UCoscaWorldSubsystem::TickServer()
{
	if (Server.IsValid())
	{
		Server->Tick();
	}
	if (ClientSocket)
	{
		ClientSocket->Tick();
	}
}

// ---- Connection ----

void UCoscaWorldSubsystem::OnClientConnected(INetworkingWebSocket* Socket)
{
	ClientSocket = Socket;
	if (ClientSocket)
	{
		ClientSocket->SetReceiveCallBack(FWebSocketPacketReceivedCallBack::CreateUObject(this, &UCoscaWorldSubsystem::OnPacketReceived));
		ClientSocket->SetSocketClosedCallBack(FWebSocketInfoCallBack::CreateUObject(this, &UCoscaWorldSubsystem::OnSocketClosed));
		UE_LOG(LogCosca, Log, TEXT("[Cosca] Cosca client connected: %s"), *ClientSocket->RemoteEndPoint(true));
	}
}

void UCoscaWorldSubsystem::OnSocketClosed()
{
	ClientSocket = nullptr;
	UE_LOG(LogCosca, Log, TEXT("[Cosca] Cosca client disconnected."));
}

void UCoscaWorldSubsystem::OnPacketReceived(void* Data, int32 DataSize)
{
	UE_LOG(LogCosca, Log, TEXT("[Cosca] OnPacketReceived: %d bytes"), DataSize);
	if (DataSize <= 0)
	{
		return;
	}
	// Raw JSON from WebSocket client.
	// libwebsocket may prepend a 4-byte size header; detect and strip it.
	const uint8* Bytes = (const uint8*)Data;
	int32 JsonLen = DataSize;
	if (DataSize > 4 && Bytes[0] != '{' && Bytes[0] != '"' && Bytes[0] != '[')
	{
		Bytes += 4;
		JsonLen = DataSize - 4;
	}

	// Convert UTF-8 bytes to FString correctly (TCHAR is 2 bytes on Windows).
	FUTF8ToTCHAR Converter((const ANSICHAR*)Bytes, JsonLen);
	FString Json(Converter.Length(), Converter.Get());
	UE_LOG(LogCosca, Log, TEXT("[Cosca] Received JSON: %s"), *Json);

	TSharedRef<TJsonReader<>> Reader = TJsonReaderFactory<>::Create(Json);
	TSharedPtr<FJsonObject> Obj;
	if (FJsonSerializer::Deserialize(Reader, Obj) && Obj.IsValid())
	{
		FCoscaMessage Msg = MessageFromJson(Obj);
		UE_LOG(LogCosca, Log, TEXT("[Cosca] Parsed command: type=%d id=%s"), (int32)Msg.Type, *Msg.Id);
		HandleCommand(Msg);
	}
	else
	{
		UE_LOG(LogCosca, Error, TEXT("[Cosca] Failed to parse JSON: %s"), *Json);
	}
}

// ---- Command dispatch ----

void UCoscaWorldSubsystem::HandleCommand(const FCoscaMessage& Message)
{
	switch (Message.Type)
	{
	case ECoscaMessageType::Spawn:
	{
		FSpawnPayload Payload;
		if (ParseSpawn(Message.Payload, Payload))
		{
			SpawnEntity(Payload);
		}
		break;
	}
	case ECoscaMessageType::Action:
	{
		FActionPayload Payload;
		if (ParseAction(Message.Payload, Payload))
		{
			HandleAction(Payload);
		}
		break;
	}
	case ECoscaMessageType::Destroy:
	{
		HandleDestroy(Message.EntityId);
		break;
	}
	case ECoscaMessageType::Ping:
	default:
	{
		FCoscaMessage Ack;
		Ack.Type = ECoscaMessageType::Pong;
		Ack.Id = Message.Id;
		Ack.EntityId = Message.EntityId;
		SendToCosca(Ack);
		break;
	}
	}
}

// ---- Spawning / Movement ----

AActor* UCoscaWorldSubsystem::SpawnEntity(const FSpawnPayload& Payload)
{
	UWorld* World = GetWorld();
	if (!World)
	{
		return nullptr;
	}

	if (AActor* Existing = FindEntity(Payload.EntityId))
	{
		Existing->SetActorLocationAndRotation(Payload.Position, Payload.Rotation, false, nullptr, ETeleportType::TeleportPhysics);
		return Existing;
	}

	UClass* SpawnClass = AActor::StaticClass();
	if (Payload.Type == TEXT("npc"))
	{
		SpawnClass = ACharacter::StaticClass();
	}
	else
	{
		SpawnClass = AStaticMeshActor::StaticClass();
	}

	FActorSpawnParameters Params;
	Params.Name = FName(*Payload.EntityId);
	// Note: SpawnActor<T>(UClass*, FVector, FRotator, Params) is not available;
	// convert quaternion to rotator for the (FVector, FRotator, Params) overload.
	AActor* Spawned = World->SpawnActor<AActor>(SpawnClass, Payload.Position, Payload.Rotation.Rotator(), Params);
	if (!Spawned)
	{
		UE_LOG(LogCosca, Error, TEXT("[Cosca] Failed to spawn entity %s"), *Payload.EntityId);
		return nullptr;
	}

	Spawned->SetActorScale3D(Payload.Scale);

	// Apply green material so entities are visible in viewport
	if (AStaticMeshActor* MeshActor = Cast<AStaticMeshActor>(Spawned))
	{
		if (UStaticMeshComponent* MeshComp = MeshActor->GetStaticMeshComponent())
		{
			// 1. Set mobility to Movable (for future move/destroy)
			MeshComp->SetMobility(EComponentMobility::Movable);

			// 2. Assign engine cube mesh (without this, nothing renders!)
			UStaticMesh* CubeMesh = LoadObject<UStaticMesh>(
				nullptr,
				TEXT("/Engine/BasicShapes/Cube.Cube"));
			if (CubeMesh)
			{
				MeshComp->SetStaticMesh(CubeMesh);
			}

			// 3. Create and apply green material
			UMaterialInstanceDynamic* Mat = UMaterialInstanceDynamic::Create(
				MeshComp->GetMaterial(0), this);
			if (Mat)
			{
				Mat->SetVectorParameterValue(FName("BaseColor"), FLinearColor(0.0f, 1.0f, 0.0f));
				MeshComp->SetMaterial(0, Mat);
			}
		}
	}

	// Attach a CoscaEntityComponent binding the Cosca id.
	UCoscaEntityComponent* Comp = NewObject<UCoscaEntityComponent>(Spawned);
	Comp->RegisterComponent();
	Comp->InitFromSpawn(Payload.EntityId, Payload.Type, Payload.AssetHash, Payload.Position, Payload.Rotation, Payload.Scale, Payload.Tags);

	EntityMap.Add(Payload.EntityId, Spawned);
	UE_LOG(LogCosca, Log, TEXT("[Cosca] Spawned %s (%s) at %s"), *Payload.EntityId, *Payload.Type, *Payload.Position.ToString());

	FCoscaMessage Ack;
	Ack.Type = ECoscaMessageType::Pong;
	Ack.EntityId = Payload.EntityId;
	SendToCosca(Ack);
	return Spawned;
}

bool UCoscaWorldSubsystem::DestroyEntity(const FString& EntityId)
{
	AActor* Actor = FindEntity(EntityId);
	if (!Actor)
	{
		return false;
	}
	Actor->Destroy();
	EntityMap.Remove(EntityId);
	return true;
}

void UCoscaWorldSubsystem::HandleDestroy(const FString& EntityId)
{
	DestroyEntity(EntityId);
}

bool UCoscaWorldSubsystem::MoveEntity(const FString& EntityId, const FVector& Target)
{
	AActor* Actor = FindEntity(EntityId);
	if (!Actor)
	{
		return false;
	}

	UNavigationSystemV1* NavSys = FNavigationSystem::GetCurrent<UNavigationSystemV1>(GetWorld());
	if (NavSys)
	{
		UNavigationPath* Path = NavSys->FindPathToLocationSynchronously(GetWorld(), Actor->GetActorLocation(), Target);
		if (Path && Path->PathPoints.Num() > 0)
		{
			Actor->SetActorLocation(Path->PathPoints.Last());
			return true;
		}
	}
	Actor->SetActorLocation(Target);
	return true;
}

void UCoscaWorldSubsystem::HandleAction(const FActionPayload& Payload)
{
	if (Payload.Action == TEXT("move_to"))
	{
		const FString* T = Payload.Params.Find(TEXT("target"));
		if (T)
		{
			FVector Target;
			// Parse "x,y,z" manually (FParse::XYZ does not exist in 5.8).
			TArray<FString> Parts;
			T->ParseIntoArray(Parts, TEXT(","), true);
			if (Parts.Num() == 3)
			{
				Target.X = FCString::Atof(*Parts[0]);
				Target.Y = FCString::Atof(*Parts[1]);
				Target.Z = FCString::Atof(*Parts[2]);
				MoveEntity(Payload.EntityId, Target);
			}
		}
	}
}

AActor* UCoscaWorldSubsystem::FindEntity(const FString& EntityId) const
{
	if (const TWeakObjectPtr<AActor>* Found = EntityMap.Find(EntityId))
	{
		return Found->Get();
	}
	return nullptr;
}

// ---- Broadcasting ----

void UCoscaWorldSubsystem::SendToCosca(const FCoscaMessage& Message)
{
	if (!ClientSocket)
	{
		return;
	}
	FString Json = MessageToJson(Message);
	// Convert FString to UTF-8 before sending (TCHAR is 2 bytes on Windows,
	// but WebSocket JSON must be UTF-8 encoded).
	FTCHARToUTF8 Converter(*Json);
	int32 Utf8Len = Converter.Length();
	TArray<uint8> Out;
	Out.Append((const uint8*)Converter.Get(), Utf8Len);
	UE_LOG(LogCosca, Log, TEXT("[Cosca] Sending to client: %s"), *Json);
	ClientSocket->Send(Out.GetData(), Out.Num(), false);
}

// ---- JSON helpers (full implementation) ----

FCoscaMessage UCoscaWorldSubsystem::MessageFromJson(const TSharedPtr<FJsonObject>& Obj)
{
	FCoscaMessage Msg;
	if (!Obj.IsValid())
	{
		return Msg;
	}
	FString TypeStr;
	if (Obj->TryGetStringField(TEXT("type"), TypeStr))
	{
		Msg.Type = TypeFromString(TypeStr);
	}
	Obj->TryGetStringField(TEXT("id"), Msg.Id);
	Obj->TryGetStringField(TEXT("entity_id"), Msg.EntityId);
	if (Obj->HasTypedField<EJson::Object>(TEXT("payload")))
	{
		TSharedPtr<FJsonObject> P = Obj->GetObjectField(TEXT("payload"));
		FString OutStr;
		TSharedRef<TJsonWriter<>> Writer = TJsonWriterFactory<>::Create(&OutStr);
		FJsonSerializer::Serialize(P.ToSharedRef(), Writer);
		Msg.Payload = OutStr;
	}
	return Msg;
}

ECoscaMessageType UCoscaWorldSubsystem::TypeFromString(const FString& S)
{
	if (S == TEXT("frame")) return ECoscaMessageType::Frame;
	if (S == TEXT("audio")) return ECoscaMessageType::Audio;
	if (S == TEXT("event")) return ECoscaMessageType::Event;
	if (S == TEXT("state_sync")) return ECoscaMessageType::StateSync;
	if (S == TEXT("action")) return ECoscaMessageType::Action;
	if (S == TEXT("spawn")) return ECoscaMessageType::Spawn;
	if (S == TEXT("destroy")) return ECoscaMessageType::Destroy;
	if (S == TEXT("modify")) return ECoscaMessageType::Modify;
	if (S == TEXT("weather")) return ECoscaMessageType::Weather;
	if (S == TEXT("time")) return ECoscaMessageType::Time;
	if (S == TEXT("pong")) return ECoscaMessageType::Pong;
	if (S == TEXT("error")) return ECoscaMessageType::Error;
	return ECoscaMessageType::Ping;
}

FString UCoscaWorldSubsystem::MessageToJson(const FCoscaMessage& Message)
{
	TSharedPtr<FJsonObject> Obj = MakeShared<FJsonObject>();
	Obj->SetStringField(TEXT("type"), TypeToString(Message.Type));
	Obj->SetStringField(TEXT("id"), Message.Id);
	Obj->SetStringField(TEXT("entity_id"), Message.EntityId);
	FString OutStr;
	TSharedRef<TJsonWriter<>> Writer = TJsonWriterFactory<>::Create(&OutStr);
	FJsonSerializer::Serialize(Obj.ToSharedRef(), Writer);
	return OutStr;
}

FString UCoscaWorldSubsystem::TypeToString(ECoscaMessageType T)
{
	switch (T)
	{
	case ECoscaMessageType::Frame: return TEXT("frame");
	case ECoscaMessageType::Audio: return TEXT("audio");
	case ECoscaMessageType::Event: return TEXT("event");
	case ECoscaMessageType::StateSync: return TEXT("state_sync");
	case ECoscaMessageType::Action: return TEXT("action");
	case ECoscaMessageType::Spawn: return TEXT("spawn");
	case ECoscaMessageType::Destroy: return TEXT("destroy");
	case ECoscaMessageType::Modify: return TEXT("modify");
	case ECoscaMessageType::Weather: return TEXT("weather");
	case ECoscaMessageType::Time: return TEXT("time");
	case ECoscaMessageType::Pong: return TEXT("pong");
	case ECoscaMessageType::Error: return TEXT("error");
	default: return TEXT("ping");
	}
}

bool UCoscaWorldSubsystem::ParseSpawn(const FString& Json, FSpawnPayload& Out)
{
	TSharedRef<TJsonReader<>> Reader = TJsonReaderFactory<>::Create(Json);
	TSharedPtr<FJsonObject> Obj;
	if (!FJsonSerializer::Deserialize(Reader, Obj) || !Obj.IsValid())
	{
		return false;
	}

	Obj->TryGetStringField(TEXT("id"), Out.EntityId);
	Obj->TryGetStringField(TEXT("type"), Out.Type);
	Obj->TryGetStringField(TEXT("asset_hash"), Out.AssetHash);

	// Position [x,y,z]
	const TArray<TSharedPtr<FJsonValue>>* Pos = nullptr;
	if (Obj->TryGetArrayField(TEXT("position"), Pos) && Pos->Num() == 3)
	{
		Out.Position.X = (*Pos)[0]->AsNumber();
		Out.Position.Y = (*Pos)[1]->AsNumber();
		Out.Position.Z = (*Pos)[2]->AsNumber();
	}
	// Scale [x,y,z]
	const TArray<TSharedPtr<FJsonValue>>* Scale = nullptr;
	if (Obj->TryGetArrayField(TEXT("scale"), Scale) && Scale->Num() == 3)
	{
		Out.Scale.X = (*Scale)[0]->AsNumber();
		Out.Scale.Y = (*Scale)[1]->AsNumber();
		Out.Scale.Z = (*Scale)[2]->AsNumber();
	}
	return true;
}

bool UCoscaWorldSubsystem::ParseAction(const FString& Json, FActionPayload& Out)
{
	TSharedRef<TJsonReader<>> Reader = TJsonReaderFactory<>::Create(Json);
	TSharedPtr<FJsonObject> Obj;
	if (!FJsonSerializer::Deserialize(Reader, Obj) || !Obj.IsValid())
	{
		return false;
	}
	Obj->TryGetStringField(TEXT("entity_id"), Out.EntityId);
	Obj->TryGetStringField(TEXT("action"), Out.Action);
	// Params (flat string map)
	TSharedPtr<FJsonObject> ParamsObj = Obj->GetObjectField(TEXT("params"));
	if (ParamsObj.IsValid())
	{
		for (const auto& Pair : ParamsObj->Values)
		{
			Out.Params.Add(FString(Pair.Key), Pair.Value->AsString());
		}
	}
	return true;
}
