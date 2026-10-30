import { expect, test } from '@grafana/plugin-e2e';

import { type PostgresOptions } from '../src/types';

const PLUGIN_TYPE = 'grafana-postgresql-datasource';

// Cloud-compatible connection defaults: fall back to local docker-compose values.
const DS_HOST = process.env.DS_INSTANCE_HOST
  ? `${process.env.DS_INSTANCE_HOST}:${process.env.DS_INSTANCE_PORT || '5432'}`
  : 'postgres:5432';
const DS_DATABASE = process.env.DS_INSTANCE_DATABASE || 'testdb';
const DS_USERNAME = process.env.DS_INSTANCE_USERNAME || 'grafana';

test.describe('Config editor', () => {
  test.describe('rendering', () => {
    test(
      'smoke: should render config editor',
      { tag: '@plugins' },
      async ({ createDataSourceConfigPage, page }) => {
        await createDataSourceConfigPage({ type: PLUGIN_TYPE });

        await expect(page.getByPlaceholder('localhost:5432')).toBeVisible();
        await expect(page.getByPlaceholder('Database')).toBeVisible();
        await expect(page.getByPlaceholder('Username')).toBeVisible();
      }
    );

    test('should render Connection section', async ({ createDataSourceConfigPage, page }) => {
      await createDataSourceConfigPage({ type: PLUGIN_TYPE });

      // Use exact:true to avoid matching the "Connection limits" subsection heading.
      await expect(page.getByRole('heading', { name: 'Connection', exact: true })).toBeVisible();
      await expect(page.getByPlaceholder('localhost:5432')).toBeVisible();
      await expect(page.getByPlaceholder('Database')).toBeVisible();
    });

    test('should render Authentication section', async ({ createDataSourceConfigPage, page }) => {
      await createDataSourceConfigPage({ type: PLUGIN_TYPE });

      await expect(page.getByRole('heading', { name: 'Authentication' })).toBeVisible();
      await expect(page.getByPlaceholder('Username')).toBeVisible();
      // TLS/SSL Mode label is visible (the Combobox has no inputId so getByRole('combobox') won't
      // match by name — verify the label is present instead).
      await expect(page.getByText('TLS/SSL Mode').first()).toBeVisible();
    });
  });

  test.describe('provisioned datasource', () => {
    test('should load provisioned connection settings', async ({
      readProvisionedDataSource,
      gotoDataSourceConfigPage,
      page,
    }) => {
      const ds = await readProvisionedDataSource<PostgresOptions>({ fileName: 'datasources.yml' });
      await gotoDataSourceConfigPage(ds.uid);

      // @grafana/ui Field without an explicit `id` on the child Input doesn't associate
      // the label via HTML for/id, so getByRole('textbox', { name }) won't match.
      // Use getByPlaceholder — it finds the element by its placeholder attribute regardless
      // of whether the field has a value.
      await expect(page.getByPlaceholder('localhost:5432')).toHaveValue(DS_HOST);
      await expect(page.getByPlaceholder('Database')).toHaveValue(DS_DATABASE);
      await expect(page.getByPlaceholder('Username')).toHaveValue(DS_USERNAME);
    });
  });

  test.describe('response size limit', () => {
    test('should render Response size limit field', async ({ createDataSourceConfigPage, page }) => {
      await createDataSourceConfigPage({ type: PLUGIN_TYPE });

      await expect(page.getByText('Response size limit')).toBeVisible();
      // No inputId on this Input, same as other fields in this file - getByPlaceholder
      // is the only reliable way to locate it (see the comment on the provisioned-datasource
      // test above). exact:true is required here - getByPlaceholder does substring
      // matching by default, and "0" would also match the neighboring "100"/"14400" fields.
      await expect(page.getByPlaceholder('0', { exact: true })).toBeVisible();
    });

    test('should save and reload the configured limit', async ({ createDataSourceConfigPage, page }) => {
      const configPage = await createDataSourceConfigPage({ type: PLUGIN_TYPE });
      await page.getByPlaceholder('localhost:5432').fill('localhost:5432');
      await page.getByPlaceholder('0', { exact: true }).fill('5000000');

      await configPage.mockHealthCheckResponse({ status: 'OK' }, 200);
      await expect(configPage.saveAndTest()).toBeOK();

      await configPage.goto();
      await expect(page.getByPlaceholder('0', { exact: true })).toHaveValue('5000000');
    });
  });

  test.describe('save & test', () => {
    test('should pass health check for provisioned datasource', async ({
      readProvisionedDataSource,
      gotoDataSourceConfigPage,
      page,
    }) => {
      const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
      const configPage = await gotoDataSourceConfigPage(ds.uid);

      // For provisioned datasources, click "Save & test" rather than using
      // configPage.saveAndTest() which can time out because it waits for a save response.
      await page.getByRole('button', { name: /save.*test/i }).click();
      await expect(configPage).toHaveAlert('success');
    });

    test('should pass health check when mocked as OK', async ({ createDataSourceConfigPage, page }) => {
      const configPage = await createDataSourceConfigPage({ type: PLUGIN_TYPE });
      await page.getByPlaceholder('localhost:5432').fill('localhost:5432');
      await configPage.mockHealthCheckResponse({ status: 'OK' }, 200);

      // toBeOK() verifies the mock response was triggered (HTTP 200).
      // In Grafana 12+ success is shown as a toast (notifyApp), not a page-level Alert,
      // so toHaveAlert('success') is not used here.
      await expect(configPage.saveAndTest()).toBeOK();
    });

    test('should show error alert when health check fails', async ({ createDataSourceConfigPage, page }) => {
      const configPage = await createDataSourceConfigPage({ type: PLUGIN_TYPE });
      await page.getByPlaceholder('localhost:5432').fill('localhost:5432');
      await configPage.mockHealthCheckResponse({ message: 'connection refused' }, 400);

      await configPage.saveAndTest();
      await expect(configPage).toHaveAlert('error');
    });

    test('should show error alert when host is unreachable', async ({ createDataSourceConfigPage, page }) => {
      const configPage = await createDataSourceConfigPage({ type: PLUGIN_TYPE });
      await page.getByPlaceholder('localhost:5432').fill('localhost:19432');

      await configPage.saveAndTest();
      await expect(configPage).toHaveAlert('error');
    });
  });
});
