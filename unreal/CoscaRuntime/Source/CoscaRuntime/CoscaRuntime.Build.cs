// CoscaRuntime.Build.cs — build dependencies for the Cosca-Living-World bridge.
using UnrealBuildTool;

public class CoscaRuntime : ModuleRules
{
	public CoscaRuntime(ReadOnlyTargetRules Target) : base(Target)
	{
		PCHUsage = ModuleRules.PCHUsageMode.UseExplicitOrSharedPCHs;

		PublicDependencyModuleNames.AddRange(new string[]
		{
			"Core",
			"CoreUObject",
			"Engine",
			"GameplayTags",
			"NavigationSystem",
			"Json",
			"JsonUtilities",
			"HTTP",
			"WebSocketNetworking",
			"AIModule",
			"InputCore",
		});

		PrivateDependencyModuleNames.AddRange(new string[]
		{
			"NetCore",
		});
	}
}
