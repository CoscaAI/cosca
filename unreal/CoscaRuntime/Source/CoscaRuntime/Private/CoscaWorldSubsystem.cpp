// CoscaWorldSubsystem.cpp
#include "CoscaWorldSubsystem.h"
#include "CoscaEntityComponent.h"
#include "Engine/World.h"
#include "Engine/Engine.h"
#include "Engine/StaticMeshActor.h"
#include "Components/StaticMeshComponent.h"
#include "Engine/StaticMesh.h"
#include "Materials/MaterialInstanceDynamic.h"
#include "Misc/Paths.h"
#include "Misc/FileHelper.h"
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
#include "Components/DirectionalLightComponent.h"
#include "Components/SkyLightComponent.h"
#include "Components/ExponentialHeightFogComponent.h"
#include "Engine/DirectionalLight.h"
#include "Engine/SkyLight.h"
#include "Engine/ExponentialHeightFog.h"
#include "EngineUtils.h"

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
	// Advance the day/night clock if auto-advance is enabled.
	if (CurrentWeather.Type.IsEmpty())
	{
		CurrentWeather.Type = TEXT("clear");
		CurrentWeather.Intensity = 1.0f;
	}
	TickTimeOfDay(DeltaTime);
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
	case ECoscaMessageType::ImportMesh:
	{
		FImportMeshPayload Payload;
		if (ParseImportMesh(Message.Payload, Payload))
		{
			ImportMesh(Payload);
		}
		break;
	}
	case ECoscaMessageType::Time:
	{
		FTimePayload Payload;
		if (ParseTime(Message.Payload, Payload))
		{
			HandleTimeOfDay(Payload);
		}
		break;
	}
	case ECoscaMessageType::Weather:
	{
		FWeatherPayload Payload;
		if (ParseWeather(Message.Payload, Payload))
		{
			HandleWeather(Payload);
		}
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

// ---- Day/Night + Weather ----

bool UCoscaWorldSubsystem::ParseTime(const FString& JsonStr, FTimePayload& Payload)
{
	TSharedRef<TJsonReader<>> Reader = TJsonReaderFactory<>::Create(JsonStr);
	TSharedPtr<FJsonObject> Obj;
	if (!FJsonSerializer::Deserialize(Reader, Obj) || !Obj.IsValid())
	{
		return false;
	}

	if (Obj->HasField(TEXT("hour")))
	{
		Payload.Hour = Obj->GetNumberField(TEXT("hour"));
	}
	return true;
}

bool UCoscaWorldSubsystem::ParseWeather(const FString& JsonStr, FWeatherPayload& Payload)
{
	TSharedRef<TJsonReader<>> Reader = TJsonReaderFactory<>::Create(JsonStr);
	TSharedPtr<FJsonObject> Obj;
	if (!FJsonSerializer::Deserialize(Reader, Obj) || !Obj.IsValid())
	{
		return false;
	}

	if (Obj->HasField(TEXT("type")))
	{
		Payload.Type = Obj->GetStringField(TEXT("type"));
	}
	if (Obj->HasField(TEXT("intensity")))
	{
		Payload.Intensity = Obj->GetNumberField(TEXT("intensity"));
	}
	return true;
}

void UCoscaWorldSubsystem::ApplyTimeOfDay(const FTimePayload& Payload)
{
	CurrentHour = FMath::Clamp(Payload.Hour, 0.0f, 24.0f);

	// Sun elevation: -90 (midnight) at 0/24, +90 (noon) at 12.
	float ElevationDeg = 90.0f * FMath::Sin((CurrentHour - 6.0f) / 12.0f * PI);
	// Sun azimuth rotates 360 across the day.
	float AzimuthDeg = CurrentHour / 24.0f * 360.0f - 180.0f;

	FVector SunDir;
	float El = FMath::DegreesToRadians(ElevationDeg);
	float Az = FMath::DegreesToRadians(AzimuthDeg);
	SunDir.X = FMath::Cos(El) * FMath::Cos(Az);
	SunDir.Y = FMath::Cos(El) * FMath::Sin(Az);
	SunDir.Z = FMath::Sin(El);
	SunDir.Normalize();

	// Directional light shines along -SunDir (from sun toward scene).
	FRotator SunRot = FRotationMatrix::MakeFromZ(SunDir).Rotator();

	// Find or create a directional light (the sun).
	ADirectionalLight* Sun = Cast<ADirectionalLight>(SunActor.Get());
	if (!Sun)
	{
		for (TActorIterator<ADirectionalLight> It(GetWorld()); It; ++It)
		{
			Sun = *It;
			break;
		}
	}
	if (!Sun)
	{
		Sun = GetWorld()->SpawnActor<ADirectionalLight>();
	}
	if (Sun)
	{
		SunActor = Sun;
		Sun->SetActorRotation(SunRot);

		if (UDirectionalLightComponent* Comp = Sun->FindComponentByClass<UDirectionalLightComponent>())
		{
			// Day factor 0(noite)-1(dia). Smooth transition across sunrise/sunset.
			float DayFactor = FMath::Clamp((ElevationDeg + 10.0f) / 20.0f, 0.0f, 1.0f);

			// Sun intensity: bright at day, 0 at night.
			Comp->SetIntensity(DayFactor * 130000.0f);

			// Color temperature: warm at dawn/dusk, white at noon, blue at night.
			FLinearColor SunColor = FLinearColor::White;
			if (DayFactor > 0.0f && DayFactor < 0.4f)
			{
				// Low angle ~ sunrise/dusk: warm orange.
				SunColor = FLinearColor(1.0f, 0.55f, 0.2f);
			}
			else if (DayFactor < 0.05f)
			{
				// Night: pale cold moonlight.
				SunColor = FLinearColor(0.15f, 0.2f, 0.4f);
			}
			else
			{
				SunColor = FLinearColor::White;
			}
			Comp->SetLightColor(SunColor);
		}
	}

	// Ambient: sky light scales with day, but stays a bit at night (moon/ambient).
	ASkyLight* SkyLight = Cast<ASkyLight>(SkyLightActor.Get());
	if (!SkyLight)
	{
		for (TActorIterator<ASkyLight> It(GetWorld()); It; ++It)
		{
			SkyLight = *It;
			break;
		}
	}
	if (SkyLight)
	{
		SkyLightActor = SkyLight;
		if (USkyLightComponent* SLComp = SkyLight->FindComponentByClass<USkyLightComponent>())
		{
			float DayFactor = FMath::Clamp((ElevationDeg + 10.0f) / 20.0f, 0.0f, 1.0f);
			// Ambient intensity: 0.05 at night, 1.0 at day (scale by weather too).
			float BaseAmbient = FMath::Lerp(0.03f, 1.0f, DayFactor) * CurrentWeather.Intensity;
			SLComp->SetIntensity(BaseAmbient * 1.2f);

			// Sky color: warm at dawn/dusk, blue at day, dark blue at night.
			FLinearColor SkyColor = FLinearColor(0.1f, 0.15f, 0.3f); // night
			if (DayFactor > 0.4f)
			{
				SkyColor = FLinearColor(0.55f, 0.75f, 1.0f); // day
			}
			else if (DayFactor > 0.05f)
			{
				SkyColor = FLinearColor(0.8f, 0.5f, 0.3f); // dusk/dawn
			}
			SLComp->SetLightColor(SkyColor);
		}
	}

	// Fog: denser/darker at night, warmer at dawn/dusk.
	AExponentialHeightFog* Fog = Cast<AExponentialHeightFog>(FogActor.Get());
	if (!Fog)
	{
		for (TActorIterator<AExponentialHeightFog> It(GetWorld()); It; ++It)
		{
			Fog = *It;
			break;
		}
	}
	if (Fog)
	{
		FogActor = Fog;
		if (UExponentialHeightFogComponent* FogComp = Fog->FindComponentByClass<UExponentialHeightFogComponent>())
		{
			float DayFactor = FMath::Clamp((ElevationDeg + 10.0f) / 20.0f, 0.0f, 1.0f);
			// Night fog is darker/denser.
			FogComp->SetFogDensity(FMath::Lerp(0.02f, 0.005f, DayFactor) * (0.5f + CurrentWeather.Intensity));
			FLinearColor FogColor = FLinearColor::White;
			if (DayFactor < 0.1f)
			{
				FogColor = FLinearColor(0.05f, 0.07f, 0.12f); // night
			}
			else if (DayFactor < 0.4f)
			{
				FogColor = FLinearColor(0.5f, 0.3f, 0.15f); // dusk
			}
			FogComp->SetFogInscatteringColor(FogColor);
		}
	}

	UE_LOG(LogCosca, Log, TEXT("[Cosca] TimeOfDay: hour=%.1f elevation=%.1f"), CurrentHour, ElevationDeg);
}

void UCoscaWorldSubsystem::HandleTimeOfDay(const FTimePayload& Payload)
{
	ApplyTimeOfDay(Payload);
}

void UCoscaWorldSubsystem::TickTimeOfDay(float DeltaTime)
{
	if (bAutoAdvanceTime && TimeScale > 0.0f)
	{
		CurrentHour += TimeScale * DeltaTime;
		if (CurrentHour >= 24.0f)
		{
			CurrentHour -= 24.0f;
		}

		FTimePayload Payload;
		Payload.Hour = CurrentHour;
		ApplyTimeOfDay(Payload);
	}
}

void UCoscaWorldSubsystem::HandleWeather(const FWeatherPayload& Payload)
{
	CurrentWeather = Payload;

	// Re-apply lighting so weather can modulate ambient/fog.
	FTimePayload TimePayload;
	TimePayload.Hour = CurrentHour;
	ApplyTimeOfDay(TimePayload);

	UE_LOG(LogCosca, Log, TEXT("[Cosca] Weather: type=%s intensity=%.2f"), *Payload.Type, Payload.Intensity);
}

void UCoscaWorldSubsystem::ApplyWeather(const FWeatherPayload& Payload)
{
	HandleWeather(Payload);
}

// ---- Import Mesh (GLB/FBX → StaticMesh → Spawn) ----

bool UCoscaWorldSubsystem::ParseImportMesh(const FString& JsonStr, FImportMeshPayload& Payload)
{
	TSharedRef<TJsonReader<>> Reader = TJsonReaderFactory<>::Create(JsonStr);
	TSharedPtr<FJsonObject> Obj;
	if (!FJsonSerializer::Deserialize(Reader, Obj) || !Obj.IsValid())
	{
		return false;
	}

	Payload.EntityId = Obj->GetStringField(TEXT("entity_id"));
	Payload.MeshPath = Obj->GetStringField(TEXT("mesh_path"));
	Payload.Type = Obj->GetStringField(TEXT("type"));

	const TArray<TSharedPtr<FJsonValue>>* PosArr;
	if (Obj->TryGetArrayField(TEXT("position"), PosArr) && PosArr->Num() == 3)
	{
		Payload.Position.X = (*PosArr)[0]->AsNumber();
		Payload.Position.Y = (*PosArr)[1]->AsNumber();
		Payload.Position.Z = (*PosArr)[2]->AsNumber();
	}

	const TArray<TSharedPtr<FJsonValue>>* ScaleArr;
	if (Obj->TryGetArrayField(TEXT("scale"), ScaleArr) && ScaleArr->Num() == 3)
	{
		Payload.Scale.X = (*ScaleArr)[0]->AsNumber();
		Payload.Scale.Y = (*ScaleArr)[1]->AsNumber();
		Payload.Scale.Z = (*ScaleArr)[2]->AsNumber();
	}

	return true;
}

AActor* UCoscaWorldSubsystem::ImportMesh(const FImportMeshPayload& Payload)
{
	UWorld* World = GetWorld();
	if (!World)
	{
		return nullptr;
	}

	UE_LOG(LogCosca, Log, TEXT("[Cosca] ImportMesh: entity=%s asset=%s"), *Payload.EntityId, *Payload.MeshPath);

	// Check if already exists
	if (AActor* Existing = FindEntity(Payload.EntityId))
	{
		Existing->SetActorLocationAndRotation(Payload.Position, Payload.Rotation, false, nullptr, ETeleportType::TeleportPhysics);
		return Existing;
	}

	// ── AssetID → Registry ──
	// Payload.MeshPath is now the AssetID (e.g. "tree_modern_001")
	// Registry maps it to a UE content path (e.g. "/Game/Cosca/Imported/tree_modern_001")
	FString ContentPath;
	
	if (Payload.MeshPath.StartsWith(TEXT("/Game/")))
	{
		// Already a content path (backward compatibility)
		ContentPath = Payload.MeshPath;
	}
	else
	{
		// It's an AssetID — read the registry
		ContentPath = ResolveAssetID(Payload.MeshPath);
		if (ContentPath.IsEmpty())
		{
			UE_LOG(LogCosca, Error, TEXT("[Cosca] ImportMesh: asset_id '%s' not found in registry"), *Payload.MeshPath);
			FCoscaMessage Ack;
			Ack.Type = ECoscaMessageType::Error;
			Ack.EntityId = Payload.EntityId;
			Ack.Payload = TEXT("{\"error\":\"asset_not_found\",\"asset_id\":\"") + Payload.MeshPath + TEXT("\"}");
			SendToCosca(Ack);
			return nullptr;
		}
	}

	UE_LOG(LogCosca, Log, TEXT("[Cosca] ImportMesh: resolved to %s"), *ContentPath);

	// Load the static mesh
	UStaticMesh* Mesh = LoadObject<UStaticMesh>(nullptr, *ContentPath);
	if (!Mesh)
	{
		// Fallback: try to use engine cube if asset not found
		Mesh = LoadObject<UStaticMesh>(nullptr, TEXT("/Engine/BasicShapes/Cube.Cube"));
		if (Mesh)
		{
			UE_LOG(LogCosca, Warning, TEXT("[Cosca] ImportMesh: asset '%s' not found, using fallback cube"), *Payload.MeshPath);
		}
		else
		{
			UE_LOG(LogCosca, Error, TEXT("[Cosca] ImportMesh: could not load mesh from %s and fallback failed"), *ContentPath);
			FCoscaMessage Ack;
			Ack.Type = ECoscaMessageType::Error;
			Ack.EntityId = Payload.EntityId;
			Ack.Payload = TEXT("{\"error\":\"mesh_load_failed\",\"content_path\":\"") + ContentPath + TEXT("\"}");
			SendToCosca(Ack);
			return nullptr;
		}
	}

	// Spawn actor with imported mesh
	AStaticMeshActor* MeshActor = World->SpawnActor<AStaticMeshActor>(
		AStaticMeshActor::StaticClass(), Payload.Position, Payload.Rotation.Rotator());
	if (!MeshActor)
	{
		UE_LOG(LogCosca, Error, TEXT("[Cosca] ImportMesh: failed to spawn actor"));
		return nullptr;
	}

	// Configure mesh component
	UStaticMeshComponent* MeshComp = MeshActor->GetStaticMeshComponent();
	MeshComp->SetMobility(EComponentMobility::Movable);
	MeshComp->SetStaticMesh(Mesh);
	MeshActor->SetActorScale3D(Payload.Scale);

	EntityMap.Add(Payload.EntityId, MeshActor);
	UE_LOG(LogCosca, Log, TEXT("[Cosca] ImportMesh: spawned %s with asset %s → %s"), *Payload.EntityId, *Payload.MeshPath, *ContentPath);

	FCoscaMessage Ack;
	Ack.Type = ECoscaMessageType::Pong;
	Ack.EntityId = Payload.EntityId;
	SendToCosca(Ack);
	return MeshActor;
}

// ── Asset Registry ──

FString UCoscaWorldSubsystem::ResolveAssetID(const FString& AssetId)
{
	// Read the asset registry JSON from Content/Cosca/asset_registry.json
	FString RegistryPath = FPaths::ProjectContentDir() + TEXT("Cosca/asset_registry.json");
	
 FString Json;
	if (!FFileHelper::LoadFileToString(Json, *RegistryPath))
	{
		UE_LOG(LogCosca, Warning, TEXT("[Cosca] Asset registry not found at %s"), *RegistryPath);
		return TEXT("");
	}

	TSharedRef<TJsonReader<>> Reader = TJsonReaderFactory<>::Create(Json);
	TSharedPtr<FJsonObject> Root;
	if (!FJsonSerializer::Deserialize(Reader, Root) || !Root.IsValid())
	{
		UE_LOG(LogCosca, Error, TEXT("[Cosca] Failed to parse asset registry"));
		return TEXT("");
	}

	TSharedPtr<FJsonObject> Assets = Root->GetObjectField(TEXT("assets"));
	if (!Assets.IsValid())
	{
		return TEXT("");
	}

	TSharedPtr<FJsonObject> Asset = Assets->GetObjectField(AssetId);
	if (!Asset.IsValid())
	{
		return TEXT("");
	}

	FString UEPath = Asset->GetStringField(TEXT("ue_asset"));
	UE_LOG(LogCosca, Log, TEXT("[Cosca] Resolved asset_id '%s' → %s"), *AssetId, *UEPath);
	return UEPath;
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
	if (S == TEXT("import_mesh")) return ECoscaMessageType::ImportMesh;
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
