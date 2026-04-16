import { expect, test } from '@grafana/plugin-e2e';

const PLUGIN_TYPE = 'grafana-postgresql-datasource';

test.describe('Config editor', () => {
  test.describe('rendering', () => {
    test(
      'smoke: should render config editor',
      { tag: '@plugins' },
      async ({ createDataSourceConfigPage, page }) => {
        await createDataSourceConfigPage({ type: PLUGIN_TYPE });

        // @grafana/ui <Field> doesn't associate labels via for/id unless the child
        // has an explicit id prop — getByLabel() returns nothing. Use placeholders instead.
        await expect(page.getByPlaceholder('localhost:5432')).toBeVisible();
        await expect(page.getByPlaceholder('Database')).toBeVisible();
        await expect(page.getByPlaceholder('Username')).toBeVisible();
      }
    );
  });

  test.describe('save & test', () => {
    test('should pass health check when mocked as successful', async ({ createDataSourceConfigPage, page }) => {
      const configPage = await createDataSourceConfigPage({ type: PLUGIN_TYPE });

      await page.getByPlaceholder('localhost:5432').fill('localhost:5432');
      await configPage.mockHealthCheckResponse({ status: 'OK' }, 200);

      await expect(configPage.saveAndTest()).toBeOK();
      await expect(configPage).toHaveAlert('success');
    });

    test('should fail health check when host is unreachable', async ({ createDataSourceConfigPage, page }) => {
      const configPage = await createDataSourceConfigPage({ type: PLUGIN_TYPE });

      await page.getByPlaceholder('localhost:5432').fill('localhost:19432');

      await configPage.saveAndTest();
      await expect(configPage).toHaveAlert('error');
    });
  });
});
