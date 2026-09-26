// force timezone to UTC to allow tests to work regardless of local timezone
// generally used by snapshots, but can affect specific tests
process.env.TZ = 'UTC';

const { grafanaESModules, nodeModulesToTransform } = require('./.config/jest/utils');

module.exports = {
  // Jest configuration provided by Grafana scaffolding
  ...require('./.config/jest.config'),
  transformIgnorePatterns: [nodeModulesToTransform([...grafanaESModules, '@marcbachmann/cel-js'])],
  transform: {
    '^.+\\.(t|j)sx?$': [
      '@swc/jest',
      {
        sourceMaps: 'inline',
        jsc: {
          parser: { syntax: 'typescript', tsx: true, decorators: false, dynamicImport: true },
          transform: { react: { runtime: 'automatic' } },
        },
      },
    ],
  },
};
