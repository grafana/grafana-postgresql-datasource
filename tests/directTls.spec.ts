import { expect, test } from '@grafana/plugin-e2e';
import { type Page } from '@playwright/test';

const PLUGIN_TYPE = 'grafana-postgresql-datasource';
const DIRECT_TLS_DS_NAMES = ['postgresql-direct-tls', 'postgresql-direct-tls-socks-proxy'];
const POSTGRES_NEGOTIATION_DS_NAME = 'postgresql-sni-proxy-postgres-negotiation';

// GRAFANA_URL is set only by the Cloud cron workflow (see .github/workflows/cron.yml).
// The data sources these tests read are local-only fixtures behind docker-compose.yaml's sni-proxy service.
const isCloudRun = !!process.env.GRAFANA_URL;

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
      range: { from: 'now-1h', to: 'now' },
    },
  };
  return `/explore?orgId=1&schemaVersion=1&panes=${encodeURIComponent(JSON.stringify(panes))}`;
}

// The Field label has no for/id association, so scope to the Field that wraps the label and the Combobox.
function negotiationCombobox(page: Page) {
  return page
    .getByText('TLS/SSL Negotiation', { exact: true })
    .locator('xpath=ancestor::label/../..')
    .getByRole('combobox');
}

test.describe('Direct TLS/SSL negotiation through an SNI-routing proxy', () => {
  test.beforeEach(() => {
    test.skip(isCloudRun, 'Depends on local provisioned data sources that are not available on Cloud.');
  });

  test.describe('config editor', () => {
    test('should default TLS/SSL Negotiation to postgres', async ({ createDataSourceConfigPage, page }) => {
      await createDataSourceConfigPage({ type: PLUGIN_TYPE });

      await expect(negotiationCombobox(page)).toHaveValue('postgres');
    });

    test('should hide TLS/SSL Negotiation when TLS/SSL Mode is disable', async ({
      readProvisionedDataSource,
      gotoDataSourceConfigPage,
      page,
    }) => {
      const ds = await readProvisionedDataSource({ fileName: 'datasources.yml', name: 'postgresql' });
      await gotoDataSourceConfigPage(ds.uid);

      await expect(page.getByText('TLS/SSL Mode').first()).toBeVisible();
      await expect(page.getByText('TLS/SSL Negotiation', { exact: true })).toHaveCount(0);
    });

    test('should connect after selecting direct', async ({ createDataSourceConfigPage, page }) => {
      const configPage = await createDataSourceConfigPage({ type: PLUGIN_TYPE });
      await page.getByPlaceholder('localhost:5432').fill('sni-proxy:5432');
      await page.getByPlaceholder('Database').fill('testdb');
      await page.getByPlaceholder('Username').fill('grafana');
      await page.getByPlaceholder('Password').fill('grafana');
      await page.getByPlaceholder('Password').blur();

      // Keyboard selection: a mouse click on an option freezes the page on Grafana 12.3.
      await negotiationCombobox(page).fill('direct');
      await negotiationCombobox(page).press('Enter');
      await expect(negotiationCombobox(page)).toHaveValue('direct');

      await expect(configPage.saveAndTest()).toBeOK();
    });
  });

  for (const name of DIRECT_TLS_DS_NAMES) {
    test(`should pass health check for ${name}`, async ({
      readProvisionedDataSource,
      gotoDataSourceConfigPage,
      page,
    }) => {
      const ds = await readProvisionedDataSource({ fileName: 'datasources.yml', name });
      const configPage = await gotoDataSourceConfigPage(ds.uid);

      await page.getByRole('button', { name: /save.*test/i }).click();
      await expect(configPage).toHaveAlert('success');
    });

    test(`should query over TLS for ${name}`, async ({ explorePage, readProvisionedDataSource, page }) => {
      const ds = await readProvisionedDataSource({ fileName: 'datasources.yml', name });

      const responsePromise = explorePage.waitForQueryDataResponse();
      await page.goto(exploreUrl(ds.uid, 'SELECT ssl FROM pg_stat_ssl WHERE pid = pg_backend_pid()'));

      const response = await responsePromise;
      expect(response.ok()).toBe(true);

      const body = await response.json();
      expect(body.results?.A?.frames?.[0]?.data?.values).toEqual([[true]]);
    });
  }

  test('should fail health check with postgres negotiation', async ({
    readProvisionedDataSource,
    gotoDataSourceConfigPage,
    page,
  }) => {
    const ds = await readProvisionedDataSource({ fileName: 'datasources.yml', name: POSTGRES_NEGOTIATION_DS_NAME });
    const configPage = await gotoDataSourceConfigPage(ds.uid);

    await page.getByRole('button', { name: /save.*test/i }).click();
    await expect(configPage).toHaveAlert('error');
  });
});
