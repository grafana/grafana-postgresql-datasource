import type { Configuration, RuleSetUseItem } from 'webpack';

import grafanaConfig, { type Env } from './.config/webpack/webpack.config';

// The source uses the automatic JSX runtime (no `import React`, `tsconfig` sets
// `"jsx": "react-jsx"`), but swc-loader defaults to the classic runtime and emits
// bare `React.createElement` calls. Since `react` is externalized, that throws
// "ReferenceError: React is not defined" at runtime. Force swc to the automatic
// runtime so JSX compiles to `react/jsx-runtime` imports instead.
const config = async (env: Env): Promise<Configuration> => {
  const baseConfig = await grafanaConfig(env);

  for (const rule of baseConfig.module?.rules ?? []) {
    if (!rule || typeof rule !== 'object') {
      continue;
    }

    const uses: RuleSetUseItem[] = Array.isArray(rule.use) ? rule.use : rule.use ? [rule.use] : [];

    for (const use of uses) {
      if (typeof use !== 'object' || typeof use.loader !== 'string' || !use.loader.includes('swc-loader')) {
        continue;
      }

      const options = (use.options ??= {}) as {
        jsc?: { transform?: { react?: Record<string, unknown> } };
      };
      options.jsc ??= {};
      options.jsc.transform ??= {};
      options.jsc.transform.react = {
        ...options.jsc.transform.react,
        runtime: 'automatic',
      };
    }
  }

  return baseConfig;
};

export default config;
