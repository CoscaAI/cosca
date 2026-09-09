# Example: Hello World Plugin

```json
// plugin.json
{
  "id": "hello-world",
  "version": "1.0.0",
  "name": "Hello World",
  "description": "A simple plugin that demonstrates the Cosca plugin API",
  "author": "Cosca Core",
  "license": "MIT",
  "entry": "index.js",
  "runtime": "nodejs",
  "permissions": ["events:publish"],
  "dependencies": { "cosca": ">=1.0.0" },
  "lifecycle": { "init": true, "start": true, "stop": true }
}
```

```javascript
// index.js
const { PluginBase } = require('@cosca/plugin-sdk');

class HelloWorldPlugin extends PluginBase {
  async init(config) {
    this.greeting = config.greeting || 'Hello from Cosca Plugin!';
    this.logger.info(`Plugin initialized with greeting: ${this.greeting}`);
  }

  async start() {
    this.logger.info('Plugin started!');
    await this.publishEvent('plugin:started', { 
      plugin: this.id,
      greeting: this.greeting 
    });
  }

  async stop() {
    this.logger.info('Plugin stopping...');
    await this.publishEvent('plugin:stopped', { plugin: this.id });
  }
}

module.exports = HelloWorldPlugin;
```

## Testing

```bash
# Install the plugin
cosca plugin install ./hello-world-plugin

# List installed plugins
cosca plugin list

# Check plugin status
cosca plugin status hello-world

# Remove plugin
cosca plugin remove hello-world
```
