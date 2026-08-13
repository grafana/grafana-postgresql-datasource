import { expect, test } from '@grafana/plugin-e2e';

import { type PostgresOptions } from '../src/types';

const PLUGIN_TYPE = 'grafana-postgresql-datasource';
const PROXIED_DS_NAME = 'postgresql-socks-proxy';

// Fixture data time range — must match tests/e2e/fixtures/schema.sql (seed 42).
const FIXTURE_FROM_ISO = '2026-03-17T21:00:00.000Z';
const FIXTURE_TO_ISO = '2026-03-18T01:00:00.000Z';

function exploreUrl(datasourceUID: string, rawSql: string): string {
  const panes = {
    '000': {
      datasource: datasourceUID,
      queries: [
        {
          refId: 'A',
          datasource: { type: PLUGIN_TYPE, uid: datasourceUID },
          rawSql,
          rawQuery: true,
          format: 'table',
        },
      ],
      range: { from: FIXTURE_FROM_ISO, to: FIXTURE_TO_ISO },
    },
  };
  return `/explore?orgId=1&schemaVersion=1&panes=${encodeURIComponent(JSON.stringify(panes))}`;
}

test.describe('Secure socks proxy', () => {
  test.describe('config editor', () => {
    test(
      'should render Secure Socks Proxy section',
      { tag: '@plugins' },
      async ({ createDataSourceConfigPage, page }) => {
        await createDataSourceConfigPage({ type: PLUGIN_TYPE });

        const heading = page.getByRole('heading', { name: 'Secure Socks Proxy' });
        await expect(heading).toBeVisible();

        const section = heading.locator('xpath=..');
        await expect(section.getByRole('switch')).not.toBeChecked();
      }
    );

    test('should load provisioned datasource with the proxy enabled', async ({
      readProvisionedDataSource,
      gotoDataSourceConfigPage,
      page,
    }) => {
      const ds = await readProvisionedDataSource<PostgresOptions>({
        fileName: 'datasources.yml',
        name: PROXIED_DS_NAME,
      });
      await gotoDataSourceConfigPage(ds.uid);

      const heading = page.getByRole('heading', { name: 'Secure Socks Proxy' });
      const section = heading.locator('xpath=..');
      await expect(section.getByRole('switch')).toBeChecked();
    });
  });

  test.describe('save & test', () => {
    test('should pass health check when routed through the proxy', async ({
      readProvisionedDataSource,
      gotoDataSourceConfigPage,
      page,
    }) => {
      const ds = await readProvisionedDataSource({ fileName: 'datasources.yml', name: PROXIED_DS_NAME });
      const configPage = await gotoDataSourceConfigPage(ds.uid);

      // For provisioned datasources, click "Save & test" rather than using
      // configPage.saveAndTest(), which can time out waiting for a save response.
      await page.getByRole('button', { name: /save.*test/i }).click();
      await expect(configPage).toHaveAlert('success');
    });
  });

  test.describe('querying with fixture data', () => {
    test('Code mode: raw SQL query returns rows over the proxied connection', async ({
      explorePage,
      readProvisionedDataSource,
      page,
    }) => {
      const ds = await readProvisionedDataSource({ fileName: 'datasources.yml', name: PROXIED_DS_NAME });

      // Register response listener before navigation — query fires on load.
      const responsePromise = explorePage.waitForQueryDataResponse();
      await page.goto(exploreUrl(ds.uid, 'SELECT * FROM metrics ORDER BY time'));

      const response = await responsePromise;
      expect(response.ok()).toBe(true);

      const body = await response.json();
      expect(body.results?.A?.frames?.length).toBeGreaterThan(0);
    });
  });
});
