// CoscaRuntimeModule.cpp — module entry point for the CoscaRuntime plugin.
// Without IMPLEMENT_MODULE the engine cannot initialize the module, which
// causes an EXCEPTION_ACCESS_VIOLATION at load time.
#include "Modules/ModuleManager.h"

IMPLEMENT_MODULE(FDefaultModuleImpl, CoscaRuntime);
