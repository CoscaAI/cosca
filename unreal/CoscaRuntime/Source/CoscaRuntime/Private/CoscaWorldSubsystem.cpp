// CoscaWorldSubsystem.cpp
#include "CoscaWorldSubsystem.h"
#include "CoscaEntityComponent.h"
#include "Engine/World.h"
#include "Engine/Engine.h"
#include "Engine/StaticMeshActor.h"
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
	Server = WsModule.CreateServer();
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
	Super::Tick(DeltaTime);
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
	if (DataSize <= 0)
	{
		return;
	}
	// libwebsocket prepends a 4-byte size header when bPrependSize=true.
	int32 Offset = (DataSize > 4) ? 4 : 0;
	const uint8* Bytes = (const uint8*)Data + Offset;
	int32 JsonLen = DataSize - Offset;

	FString Json(JsonLen, (const TCHAR*)Bytes);
	TSharedRef<TJsonReader<>> Reader = TJsonReaderFactory<>::Create(Json);
	TSharedPtr<FJsonObject> Obj;
	if (FJsonSerializer::Deserialize(Reader, Obj) && Obj.IsValid())
	{
		HandleCommand(MessageFromJson(Obj));
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
	AActor* Spawned = World->SpawnActor<AActor>(SpawnClass, Payload.Position, Payload.Rotation, Params);
	if (!Spawned)
	{
		UE_LOG(LogCosca, Error, TEXT("[Cosca] Failed to spawn entity %s"), *Payload.EntityId);
		return nullptr;
	}

	Spawned->SetActorScale3D(Payload.Scale);

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
		FString* T = Payload.Params.Find(TEXT("target"));
		if (T)
		{
			FVector Target;
			if (FParse::XYZ(**T, Target.X, Target.Y, Target.Z))
			{
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
	int32 Len = Json.Len();
	TArray<uint8> Out;
	for (int32 i = 0; i < 4; ++i)
	{
		Out.Add((uint8)((Len >> (8 * i)) & 0xFF));
	}
	Out.Append((const uint8*)GetData(Json), Len);
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
	TArray<TSharedPtr<FJsonValue>>* Pos = nullptr;
	if (Obj->TryGetArrayField(TEXT("position"), Pos) && Pos->Num() == 3)
	{
		Out.Position.X = (*Pos)[0]->AsNumber();
		Out.Position.Y = (*Pos)[1]->AsNumber();
		Out.Position.Z = (*Pos)[2]->AsNumber();
	}
	// Scale [x,y,z]
	TArray<TSharedPtr<FJsonValue>>* Scale = nullptr;
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
			Out.Params.Add(Pair.Key, Pair.Value->AsString());
		}
	}
	return true;
}
