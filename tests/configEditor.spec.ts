import { expect, test } from '@grafana/plugin-e2e';

const PLUGIN_TYPE = 'grafana-postgresql-datasource';

test.describe('Config editor', () => {
  test.describe('rendering', () => {
    test(
      'smoke: should render config editor',
      { tag: '@plugins' },
      async ({ createDataSourceConfigPage, page }) => {
        await createDataSourceConfigPage({ type: PLUGIN_TYPE });

        await expect(page.getByLabel('Host URL')).toBeVisible();
        await expect(page.getByLabel('Database name')).toBeVisible();
        await expect(page.getByLabel('Username')).toBeVisible();
      }
    );
  });

  test.describe('save & test', () => {
    test('should pass health check when mocked as successful', async ({ createDataSourceConfigPage, page }) => {
      const configPage = await createDataSourceConfigPage({ type: PLUGIN_TYPE });

      await page.getByLabel('Host URL').fill('localhost:5432');
      await configPage.mockHealthCheckResponse({ status: 'OK' }, 200);

      await expect(configPage.saveAndTest()).toBeOK();
      await expect(configPage).toHaveAlert('success');
    });

    test('should fail health check when host is unreachable', async ({ createDataSourceConfigPage, page }) => {
      const configPage = await createDataSourceConfigPage({ type: PLUGIN_TYPE });

      await page.getByLabel('Host URL').fill('localhost:19432');

      await configPage.saveAndTest();
      await expect(configPage).toHaveAlert('error');
    });
  });
});
