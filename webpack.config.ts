import type { Configuration } from 'webpack';
import { merge } from 'webpack-merge';
import path from 'path';
import { SOURCE_DIR } from './.config/bundler/constants';
import grafanaConfig from './.config/webpack/webpack.config';

const config = async (env): Promise<Configuration> => {
  const baseConfig = await grafanaConfig(env);

  // webpack-merge appends these rules to the base config's rules (it does NOT
  // re-spread them, which would duplicate every base rule and double-process
  // e.g. monaco-editor CSS). The appended swc-loader rule also matches .tsx and
  // runs before the base swc rule, converting JSX with the automatic runtime.
  return merge(baseConfig, {
    module: {
      rules: [
        {
          exclude: /(node_modules)/,
          test: /\.[tj]sx?$/,
          use: {
            loader: 'swc-loader',
            options: {
              jsc: {
                baseUrl: path.resolve(process.cwd(), SOURCE_DIR),
                target: 'es2015',
                loose: false,
                parser: {
                  syntax: 'typescript',
                  tsx: true,
                  decorators: false,
                  dynamicImport: true,
                },
                transform: {
                  react: {
                    // The externalized source is written for the automatic JSX
                    // runtime (tsconfig "jsx": "react-jsx") and does not import
                    // React in every component. swc defaults to the classic
                    // runtime, which would emit React.createElement and crash
                    // at render with "React is not defined". Overriding here (in
                    // the root config, not the managed .config base) keeps the
                    // fix in the CI build path: package.json build/dev point at
                    // this file.
                    runtime: 'automatic',
                  },
                },
              },
            },
          },
        },
      ],
    },
    output: {
      asyncChunks: true,
    },
  });
};

export default config;
