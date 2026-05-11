import { test, expect } from '@grafana/plugin-e2e';
import { PostgresOptions, SecureJsonData } from '../src/types';

const DS_FILE = 'datasources.yml';

test.describe('Config editor', () => {
  test.describe.configure({ mode: 'serial' });

  test.describe('rendering', () => {
    test(
      'smoke: should render config editor',
      { tag: '@plugins' },
      async ({ createDataSourceConfigPage, readProvisionedDataSource, page }) => {
        const ds = await readProvisionedDataSource({ fileName: DS_FILE });
        await createDataSourceConfigPage({ type: ds.type });
        await expect(page.getByRole('heading', { name: 'Connection', exact: true })).toBeVisible();
      }
    );

    test('should render Connection section', async ({ createDataSourceConfigPage, readProvisionedDataSource, page }) => {
      const ds = await readProvisionedDataSource({ fileName: DS_FILE });
      await createDataSourceConfigPage({ type: ds.type });
      await expect(page.getByRole('heading', { name: 'Connection', exact: true })).toBeVisible();
      await expect(page.getByPlaceholder('localhost:5432')).toBeVisible();
      await expect(page.getByPlaceholder('Database')).toBeVisible();
    });

    test('should render Authentication section', async ({ createDataSourceConfigPage, readProvisionedDataSource, page }) => {
      const ds = await readProvisionedDataSource({ fileName: DS_FILE });
      await createDataSourceConfigPage({ type: ds.type });
      await expect(page.getByRole('heading', { name: 'Authentication', exact: true })).toBeVisible();
      await expect(page.getByRole('textbox', { name: 'Username' })).toBeVisible();
      await expect(page.getByText('TLS/SSL Mode').first()).toBeVisible();
    });

    test('should render Additional settings section', async ({ createDataSourceConfigPage, readProvisionedDataSource, page }) => {
      const ds = await readProvisionedDataSource({ fileName: DS_FILE });
      await createDataSourceConfigPage({ type: ds.type });
      await expect(page.getByRole('heading', { name: 'Additional settings', exact: true })).toBeVisible();
      await expect(page.getByText('TimescaleDB').first()).toBeVisible();
    });
  });

  test.describe('provisioned datasource', () => {
    test('should load provisioned connection settings', async ({
      readProvisionedDataSource,
      gotoDataSourceConfigPage,
      page,
    }) => {
      const ds = await readProvisionedDataSource<PostgresOptions, SecureJsonData>({ fileName: DS_FILE });
      await gotoDataSourceConfigPage(ds.uid);
      await expect(page.getByPlaceholder('localhost:5432')).toHaveValue('postgres:5432');
      await expect(page.getByRole('textbox', { name: 'Username' })).toHaveValue('grafana');
    });
  });

  test.describe('save & test', () => {
    test('should pass health check for provisioned datasource', async ({
      readProvisionedDataSource,
      gotoDataSourceConfigPage,
      page,
    }) => {
      const ds = await readProvisionedDataSource({ fileName: DS_FILE });
      await gotoDataSourceConfigPage(ds.uid);
      await page.getByRole('button', { name: /^(Save & test|Test)$/ }).click();
      await expect(page.getByText('Database Connection OK')).toBeVisible();
    });

    test('should show error alert when credentials are invalid', async ({
      createDataSourceConfigPage,
      readProvisionedDataSource,
      page,
    }) => {
      const ds = await readProvisionedDataSource({ fileName: DS_FILE });
      const configPage = await createDataSourceConfigPage({ type: ds.type });
      const host = process.env.DS_INSTANCE_HOST ?? 'postgres';
      const port = process.env.DS_INSTANCE_PORT ?? '5432';
      await page.getByPlaceholder('localhost:5432').fill(`${host}:${port}`);
      await page.getByPlaceholder('Database').fill(process.env.DS_INSTANCE_DATABASE ?? 'grafana');
      await page.getByRole('textbox', { name: 'Username' }).fill(process.env.DS_INSTANCE_USERNAME ?? 'grafana');
      await page.getByRole('textbox', { name: 'Password' }).fill('wrong-password');
      await expect(configPage.saveAndTest()).not.toBeOK();
      await expect(configPage).toHaveAlert('error');
    });

    test('should show error alert when host is unreachable', async ({
      createDataSourceConfigPage,
      readProvisionedDataSource,
      page,
    }) => {
      const ds = await readProvisionedDataSource({ fileName: DS_FILE });
      const configPage = await createDataSourceConfigPage({ type: ds.type });
      await page.getByPlaceholder('localhost:5432').fill('unreachable-host-e2e:5432');
      await page.getByPlaceholder('Database').fill('grafana');
      await page.getByRole('textbox', { name: 'Username' }).fill('grafana');
      await page.getByRole('textbox', { name: 'Password' }).fill('grafana');
      await expect(configPage.saveAndTest()).not.toBeOK();
      await expect(configPage).toHaveAlert('error');
    });
  });
});
