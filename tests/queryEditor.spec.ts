import { expect, test } from '@grafana/plugin-e2e';

// Fixture data time range — must match tests/e2e/fixtures/schema.sql (seed 42).
const FIXTURE_FROM_ISO = '2026-03-17T21:00:00.000Z';
const FIXTURE_TO_ISO = '2026-03-18T01:00:00.000Z';

/**
 * Builds a Grafana Explore URL with the given datasource, query state, and time range.
 * The query fires automatically on page load, so callers must register
 * `explorePage.waitForQueryDataResponse()` **before** calling `page.goto()`.
 */
function exploreUrl(
  datasourceUID: string,
  query: Record<string, unknown> = {},
  from = FIXTURE_FROM_ISO,
  to = FIXTURE_TO_ISO
): string {
  const panes = {
    '000': {
      datasource: datasourceUID,
      queries: [
        {
          refId: 'A',
          datasource: { type: 'grafana-postgresql-datasource', uid: datasourceUID },
          ...query,
        },
      ],
      range: { from, to },
    },
  };
  return `/explore?orgId=1&schemaVersion=1&panes=${encodeURIComponent(JSON.stringify(panes))}`;
}

test.describe('Query editor', () => {
  // Navigate via exploreUrl so the datasource is encoded in the URL and the query
  // editor renders on page load — no panelEditPage dashboard flow needed.
  test.beforeEach(async ({ readProvisionedDataSource, page }) => {
    const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
    await page.goto(exploreUrl(ds.uid));
  });

  test.describe('rendering', () => {
    test(
      'smoke: should render Builder and Code mode options',
      { tag: '@plugins' },
      async ({ page }) => {
        await expect(page.getByRole('radio', { name: 'Builder' })).toBeVisible();
        await expect(page.getByRole('radio', { name: 'Code' })).toBeVisible();
      }
    );
  });

  test.describe('Builder mode', () => {
    test('should activate Builder mode and show visual query builder', async ({ page }) => {
      await page.getByRole('radio', { name: 'Builder' }).click();
      await expect(page.getByRole('radio', { name: 'Builder' })).toBeChecked();

      // @grafana/sql Builder mode shows a Table combobox for selecting a table.
      await expect(page.getByRole('combobox', { name: 'Table' })).toBeVisible();
    });
  });

  test.describe('Code mode', () => {
    test('should activate Code mode and show Monaco SQL editor', async ({ page }) => {
      await page.getByRole('radio', { name: 'Code' }).click();
      await expect(page.getByRole('radio', { name: 'Code' })).toBeChecked();

      // @grafana/sql uses @grafana/plugin-ui SQLEditor → Monaco (not CodeMirror).
      // Use [role="code"] to scope away from the rename-box widget that also has
      // class="monaco-editor" in the DOM.
      await expect(page.locator('[role="code"]')).toBeVisible();
    });

    test('should accept a raw SQL query in Code mode', async ({ page }) => {
      await page.getByRole('radio', { name: 'Code' }).click();
      await expect(page.getByRole('radio', { name: 'Code' })).toBeChecked();

      const editor = page.locator('[role="code"]');
      await editor.click();
      await page.keyboard.press('ControlOrMeta+a');
      await page.keyboard.type('SELECT * FROM metrics ORDER BY time LIMIT 5');

      // Verify text is present in the editor
      await expect(editor).toContainText('SELECT');
    });
  });
});

test.describe('Query editor with fixture data', () => {
  test.describe.configure({ mode: 'serial' });

  test.describe('metrics table', () => {
    test('Code mode: raw SQL query returns rows', async ({ explorePage, readProvisionedDataSource, page }) => {
      const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });

      // Register response listener before navigation — query fires on load.
      const responsePromise = explorePage.waitForQueryDataResponse();
      await page.goto(
        exploreUrl(ds.uid, {
          rawSql: 'SELECT * FROM metrics ORDER BY time',
          rawQuery: true,
          format: 'table',
        })
      );

      const response = await responsePromise;
      expect(response.ok()).toBe(true);

      // Read body inside then() to avoid CDP buffer eviction on slow connections.
      const body = await response.json();
      expect(body.results?.A?.frames?.length).toBeGreaterThan(0);
    });

    test('Code mode: WHERE filter returns subset of rows', async ({ explorePage, readProvisionedDataSource, page }) => {
      const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });

      const responsePromise = explorePage.waitForQueryDataResponse();
      await page.goto(
        exploreUrl(ds.uid, {
          rawSql: "SELECT * FROM metrics WHERE host = 'host-a' ORDER BY time",
          rawQuery: true,
          format: 'table',
        })
      );

      const response = await responsePromise;
      expect(response.ok()).toBe(true);
      const body = await response.json();
      expect(body.results?.A?.frames?.length).toBeGreaterThan(0);
    });

    test('Code mode: aggregation query returns results', async ({ explorePage, readProvisionedDataSource, page }) => {
      const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });

      const responsePromise = explorePage.waitForQueryDataResponse();
      await page.goto(
        exploreUrl(ds.uid, {
          rawSql: "SELECT host, AVG(value) AS avg_value FROM metrics GROUP BY host ORDER BY host",
          rawQuery: true,
          format: 'table',
        })
      );

      const response = await responsePromise;
      expect(response.ok()).toBe(true);
      const body = await response.json();
      expect(body.results?.A?.frames?.length).toBeGreaterThan(0);
    });
  });
});
