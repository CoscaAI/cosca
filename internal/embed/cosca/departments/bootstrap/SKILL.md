# BOOTSTRAP CHIEF — Project Initialization & Scaffolding
- **Reports To**: Kernel
> **Version**: 1.0.0 | **Type**: kernel

## PURPOSE
Automatic workspace initialization on startup. Detect stack, create context, load memory, activate agents. Run by the Kernel when entering a new project.

## FLOW
1. Detect language, framework, database, architecture
2. Create `.cosca/` directory structure
3. Load knowledge base from embed
4. Index project files
5. Activate relevant agents based on detected stack
6. Run `cosca knowledge readiness --detect`
