# Troubleshooting: Common Issues

> **Status**: active | **Owner**: QA Chief | **Last Updated**: 2026-07-23

This guide covers common issues encountered when installing, configuring, and using Cosca, along with their solutions.

---

## Installation Issues

### Binary Not Found

**Error:**
```bash
$ cosca
command not found: cosca
```

**Causes:**
- Binary not installed
- Binary not in `$PATH`
- Incorrect installation path

**Solutions:**

```bash
# Verify installation location
which cosca || ls /usr/local/bin/cosca || ls ~/go/bin/cosca

# Add to PATH
export PATH=$PATH:$(go env GOPATH)/bin

# Reinstall
go install github.com/CoscaAI/cosca/cmd/cosca@latest

# Or copy to system path
sudo cp bin/cosca /usr/local/bin/
```

### Permission Denied

**Error:**
```bash
$ cosca
-bash: /usr/local/bin/cosca: Permission denied
```

**Solution:**
```bash
chmod +x /usr/local/bin/cosca
```

### Version Mismatch

**Error:**
```bash
$ cosca version
Error: database schema version mismatch
```

**Solution:**
```bash
# Rebuild the knowledge index
cosca knowledge rebuild

# If persists, reinstall fully
cosca uninstall
cosca install
```

### npm Installation Errors

**Error:**
```bash
npm ERR! code EACCES
npm ERR! syscall access
```

**Solutions:**
```bash
# Fix npm permissions
sudo npm install -g cosca

# Or use nvm to avoid permission issues
nvm use 18
npm install -g cosca
```

---

## Editor Integration Issues

### OpenCode Config Not Found

**Error:**
```bash
$ cosca editor detect
Detected editor: none
```

**Causes:**
- OpenCode not installed
- `.opencode/` directory does not exist
- Custom OpenCode configuration path

**Solutions:**

```bash
# Check if OpenCode is installed
which opencode

# Create .opencode directory manually
mkdir -p .opencode

# Re-run editor setup with verbose output
cosca editor detect --verbose
cosca editor setup
```

### Claude Code Not Detected

**Error:**
```bash
$ cosca editor detect
No Claude Code installation found
```

**Causes:**
- Claude Code is not installed
- `CLAUDE.md` file does not exist in project root
- Claude Code installed in a nonstandard location

**Solutions:**

```bash
# Install Claude Code
npm install -g @anthropic-ai/claude-code

# Verify installation
claude --version

# Create CLAUDE.md
echo "# Claude Code Configuration" > CLAUDE.md

# Re-run setup
cosca editor setup claude
```

### MCP Server Connection Failed

**Error:**
```bash
MCP server connection failed: dial tcp 127.0.0.1:0: connect: connection refused
```

**Causes:**
- Runtime not running in daemon mode
- MCP port already in use
- Firewall blocking the connection

**Solutions:**

```bash
# Start runtime in daemon mode
cosca runtime start

# Check if port is available
lsof -i :<port>

# Specify a different port
cosca editor setup --mcp-port 14120

# Verify MCP server is running
cosca doctor
```

### Generic MCP Fallback Not Working

**Error:**
```bash
MCP tool not found: cosca_knowledge_search
```

**Solutions:**

```bash
# Ensure runtime is running
cosca runtime status

# Verify MCP configuration in your editor
# OpenCode: check .opencode/AGENTS.md
# VS Code: check .vscode/settings.json
# Claude Code: check CLAUDE.md

# Re-run MCP-only setup
cosca editor setup generic-mcp --mcp-only
```

---

## Knowledge Engine Issues

### Index Not Building

**Error:**
```bash
$ cosca knowledge index ./docs
Error: indexing failed: no documents found
```

**Causes:**
- Path does not exist
- No supported file types in the directory
- Permission denied on files

**Solutions:**

```bash
# Verify the path exists
ls -la ./docs

# Check supported file types (must be .md, .go, .ts, .py, .js, etc.)
find ./docs -type f | head -20

# Run with verbose logging
cosca --verbose knowledge index ./docs

# Check file permissions
ls -la ./docs/*.md
```

### Search Returning No Results

**Error:**
```bash
$ cosca knowledge search "authentication"
No results found
```

**Causes:**
- Knowledge base is empty (not yet indexed)
- Index is out of date
- Query terms do not match indexed content
- Database corruption

**Solutions:**

```bash
# Check if knowledge base has data
cosca knowledge stats

# Rebuild the index
cosca knowledge rebuild

# Try a broader search
cosca knowledge search "auth"

# Use FTS5 wildcard
cosca knowledge search "auth*"

# Sync filesystem changes
cosca knowledge sync

# Verify database integrity
cosca knowledge verify

# Vacuum and rebuild
cosca knowledge vacuum
cosca knowledge rebuild
```

### Slow Search Performance

**Symptom:** Search takes >1 second

**Causes:**
- Large unindexed dataset
- Database fragmentation
- Insufficient memory cache

**Solutions:**

```bash
# Check database size
cosca knowledge stats

# Vacuum to optimize database
cosca knowledge vacuum

# Rebuild indexes
cosca knowledge rebuild

# Check system resources
cosca doctor

# Increase cache size in config
# .cosca/config.yaml
# knowledge:
#   cache_size: 512  # MB
```

### High Memory Usage

**Symptom:** Cosca consuming excessive memory

**Causes:**
- Large vector database loaded into memory
- Memory leak in plugin
- Excessive caching

**Solutions:**

```bash
# Check memory usage
cosca status --json | jq .memory

# Clear cache
cosca cache clear

# Reduce cache size in config
# .cosca/config.yaml
# cache:
#   max_size: 128  # MB

# Disable vector search if not needed
cosca knowledge search "query" --mode fts5

# Restart runtime
cosca runtime restart

# Profile memory
cosca doctor --profile
```

---

## Performance Issues

### Slow Runtime Startup

**Symptom:** `cosca status` takes >5 seconds to respond

**Causes:**
- Large plugin set
- Slow editor detection
- Database initialization on large datasets

**Solutions:**

```bash
# Run in daemon mode (start once, reuse)
cosca runtime start

# List and disable unused plugins
cosca plugin list
cosca plugin disable unused-plugin

# Check startup time breakdown
cosca --verbose status

# Reduce the number of watched directories
# .cosca/config.yaml
# watcher:
#   enabled: true
#   paths:
#     - ./docs
#     - ./src
```

### Slow Command Execution

**Symptom:** Every command takes >1 second

**Causes:**
- Runtime starting/stopping per command (CLI mode)
- File watcher processing large change sets
- Synchronous plugin hooks

**Solutions:**

```bash
# Use daemon mode for faster commands
cosca runtime start
cosca status    # Now runs in <100ms

# Check plugin hook performance
cosca plugin list --verbose

# Disable file watcher for large projects
cosca --config config-no-watcher.yaml status
```

---

## Plugin Issues

### Plugin Not Loading

**Error:**
```bash
$ cosca plugin install ./my-plugin
Error: plugin failed to load
```

**Causes:**
- Missing or invalid `plugin.yaml` manifest
- Incompatible Go version (for Go plugins)
- Missing WASM runtime (for WASM plugins)
- Dependency resolution failure

**Solutions:**

```bash
# Validate the plugin manifest
cosca plugin validate ./my-plugin/plugin.yaml

# Check Go version compatibility
go version
# Go plugins must be compiled with the same Go version as Cosca

# Verify WASM runtime
tinygo version  # or wasmtime --version

# Check plugin dependencies
cosca plugin list --dependencies

# View detailed error logs
cosca --verbose plugin install ./my-plugin
```

### Permission Errors

**Error:**
```bash
Plugin "my-plugin" attempted to access filesystem without permission
```

**Causes:**
- Plugin manifest does not declare required permissions
- Plugin is attempting operations outside its declared scope

**Solutions:**

```bash
# Check plugin manifest permissions
cosca plugin info my-plugin

# Update plugin manifest with required permissions
# plugin.yaml
# permissions:
#   - filesystem
#   - network

# Reinstall the plugin
cosca plugin remove my-plugin
cosca plugin install ./my-plugin
```

### Plugin Causing Crashes

**Symptom:** Cosca crashes when a specific plugin is active

**Causes:**
- Go native plugin causing a panic
- WASM plugin exceeding memory limits
- External plugin process crashing

**Solutions:**

```bash
# Disable the problematic plugin
cosca plugin disable my-plugin

# Check runtime logs for error details
cosca doctor --logs

# Run the plugin in a more isolated runtime
# Change runtime type in plugin.yaml:
# runtime: wasm  # instead of go

# Report the issue to the plugin author with logs
```

### Dependency Resolution Failures

**Error:**
```bash
Error: plugin "my-plugin" requires "cosca-core >= 2.0.0" but version 1.5.0 is installed
```

**Solutions:**

```bash
# List all installed plugins and versions
cosca plugin list

# Check for updates
cosca plugin update --all

# Install specific dependency version
cosca plugin install cosca-core@2.0.0
```

---

## FAQ

### General

**Q: How do I completely uninstall Cosca?**

```bash
cosca uninstall          # Removes all Cosca artifacts
rm -rf ~/.config/cosca   # Removes global configuration
```

**Q: Can I use Cosca offline?**

Yes. Cosca is designed for local-first operation. All features work offline except remote embedding providers (if configured).

**Q: How do I update Cosca?**

```bash
cosca update            # Auto-update
go install github.com/CoscaAI/cosca/cmd/cosca@latest  # Manual update
```

**Q: Does Cosca collect telemetry?**

No. Cosca does not collect any usage data or telemetry.

### Knowledge Engine

**Q: How large can the knowledge base be?**

The knowledge base can handle 100K+ documents with tens of thousands of vectors. Performance degrades gracefully beyond that.

**Q: How do I exclude files from indexing?**

Configure exclusion patterns in `.cosca/config.yaml`:
```yaml
knowledge:
  exclude_patterns:
    - "node_modules/**"
    - ".git/**"
    - "vendor/**"
```

**Q: Can I use a remote embedding model?**

Yes. Configure an embedding provider in `.cosca/config.yaml`:
```yaml
knowledge:
  embedding_provider: openai
  embedding_model: text-embedding-3-small
```

### Plugins

**Q: Can I write plugins in languages other than Go?**

Yes. Use the **External** runtime type for plugins in TypeScript, Python, Rust, or any language that can communicate via gRPC.

**Q: How do I debug a plugin?**

```bash
cosca --verbose plugin start my-plugin  # Enable verbose logging
cosca doctor --plugin my-plugin         # Plugin diagnostics
```

**Q: Are plugins sandboxed?**

WASM and External runtimes are sandboxed. Go native plugins run in-process and are intended for trusted code only.

### Editor Integration

**Q: My editor is not listed in the detected editors.**

Run `cosca editor detect --verbose` to see all detection attempts. If your editor supports MCP, use the Generic MCP fallback.

**Q: How do I manually configure MCP?**

See the [MCP Server Example](../../examples/editors/mcp-server.md) for manual configuration instructions.

**Q: Can I use Cosca without any editor integration?**

Yes. Cosca works as a standalone CLI tool without editor integration. Editor integration is optional and enhances the experience.

---

**Related**: [CLI Overview](../cli/overview.md) | [Editor Overview](../editors/overview.md) | [Knowledge Overview](../knowledge/overview.md) | [Plugin Overview](../plugins/overview.md)
