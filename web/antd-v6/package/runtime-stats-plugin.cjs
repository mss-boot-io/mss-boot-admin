const { mkdirSync, writeFileSync } = require('node:fs');
const { join } = require('node:path');

/** Emits the minimal module graph consumed by the runtime-contract gate. */
class RuntimeStatsPlugin {
  apply(compiler) {
    compiler.hooks.done.tap('MssAdminRuntimeStatsPlugin', (stats) => {
      const snapshot = stats.toJson({ all: false, modules: true, nestedModules: true });
      mkdirSync(compiler.outputPath, { recursive: true });
      writeFileSync(
        join(compiler.outputPath, 'stats.json'),
        JSON.stringify({ modules: snapshot.modules ?? [] }),
        'utf8',
      );
    });
  }
}

module.exports = RuntimeStatsPlugin;
