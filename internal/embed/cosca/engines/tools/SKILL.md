> **Version**: 1.0.0 | **Status**: active | **Owner**: Tools Engine | **Last Updated**: 2026-07-10

# TOOLS ENGINE

## PURPOSE
The Tools Engine manages the tool catalog available to agents. It registers, discovers, validates, and routes tools across the system.

## TOOL CATEGORIES

### File Operations
- read_file(path, offset?, limit?)
- write_file(path, content)
- edit_file(path, old_string, new_string)
- list_directory(path)
- find_files(pattern)
- search_content(pattern, path?)

### Code Execution
- execute_command(command, workdir?)
- run_tests(test_pattern?)
- run_linter(file?)
- run_typecheck()
- run_build()

### Git Operations
- git_status()
- git_diff(staged?)
- git_log(count?)
- git_branch()
- git_checkout(branch)
- git_commit(message)
- git_push()

### Project Operations
- install_dependencies()
- update_dependencies()
- add_dependency(name, version?)
- remove_dependency(name)

### Memory Operations
- store_memory(type, key, data)
- retrieve_memory(type, key?)
- search_memory(query, types?)
- list_memories(type?)

### Documentation Operations
- generate_readme()
- generate_api_docs()
- generate_adr(title, context, decision)
- update_changelog(entry)
- generate_release_notes(version)

### Quality Operations
- run_security_scan()
- run_performance_benchmark()
- check_test_coverage()
- run_complexity_analysis()
- check_dependency_health()

### Workflow Operations
- create_workflow(definition)
- execute_workflow(name, inputs)
- get_workflow_status(id)
- cancel_workflow(id)

### Agent Operations
- spawn_agent(type, task)
- get_agent_status(id)
- kill_agent(id)
- list_agents()

## TOOL REGISTRATION

```yaml
tool:
  name: tool_name
  description: What the tool does
  category: file|code|git|project|memory|doc|quality|workflow|agent
  parameters:
    - name: param_name
      type: string|number|boolean|object|array
      required: true|false
      description: What this parameter is
  returns:
    type: string|object|void
    description: What the tool returns
  permissions:
    - read
    - write
    - execute
  timeout_ms: 30000
```

## TOOL PERMISSIONS

| Category | Tools | Permission Level |
|----------|-------|-----------------|
| Read | read_file, list_directory, find_files, search_content | Low |
| Read+Analyze | git_status, git_diff, git_log, check_test_coverage | Low |
| Write (safe) | write_file, edit_file, generate_readme, generate_api_docs | Medium |
| Write (project) | add_dependency, remove_dependency | Medium |
| Execute (safe) | run_linter, run_typecheck, run_tests, run_build | Medium |
| Execute (unsafe) | execute_command (arbitrary) | High |
| Git write | git_commit, git_push, git_checkout | High |
| System | install_dependencies, update_dependencies | High |
| Agent Control | spawn_agent, kill_agent | Admin |

## DEPENDENCIES
- Used by all agents for task execution
- Managed by Automation Chief
- Permissions enforced by Kernel
- Audited by Audit Engine

## RELATED
- [Audit Engine](../audit/SKILL.md) — Audits all tool usage across the system
- [Runtime Engine](../runtime/SKILL.md) — Some tools target the runtime environment

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Refactor | Added metadata, HISTORY, and cross-references |
