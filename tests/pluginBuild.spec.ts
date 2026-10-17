import { expect, test } from '@grafana/plugin-e2e';

import { servesCorePlugin } from './pluginBuild';

const PLUGIN_TYPE = 'grafana-postgresql-datasource';

// GRAFANA_URL is set only by the Cloud cron workflow (see .github/workflows/cron.yml).
const isCloudRun = !!process.env.GRAFANA_URL;

test('should serve the core plugin only on Grafana versions that ship it', async ({ grafanaVersion, request }) => {
  test.skip(isCloudRun, 'Cloud serves the plugin version installed on the stack, not this build.');

  const response = await request.get(`/api/plugins/${PLUGIN_TYPE}/settings`);
  expect(response.ok()).toBe(true);

  const { signature } = await response.json();
  expect(signature === 'internal').toBe(servesCorePlugin(grafanaVersion));
});
