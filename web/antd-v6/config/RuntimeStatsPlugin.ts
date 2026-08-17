import { writeFileSync } from 'node:fs';
import { join } from 'node:path';

interface ModuleStat {
  modules?: ModuleStat[];
  name?: string;
}

interface StatsSnapshot {
  modules?: ModuleStat[];
}

interface WebpackStats {
  toJson(options: Record<string, boolean>): StatsSnapshot;
}

interface WebpackCompiler {
  hooks: {
    done: {
      tap(name: string, callback: (stats: WebpackStats) => void): void;
    };
  };
  outputPath: string;
}

/** Emits the minimal module graph consumed by the V6 runtime-contract gate. */
export default class RuntimeStatsPlugin {
  apply(compiler: WebpackCompiler): void {
    compiler.hooks.done.tap('MssV6RuntimeStatsPlugin', (stats) => {
      const snapshot = stats.toJson({ all: false, modules: true, nestedModules: true });
      writeFileSync(
        join(compiler.outputPath, 'stats.json'),
        JSON.stringify({ modules: snapshot.modules ?? [] }),
        'utf8',
      );
    });
  }
}
