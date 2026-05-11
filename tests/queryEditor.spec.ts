import { test, expect, ExplorePage } from '@grafana/plugin-e2e';

const DS_UID = 'postgresql-e2e';
const PLUGIN_ID = 'grafana-postgresql-datasource';
const FIXTURE_FROM_ISO = '2026-03-17T21:00:00.000Z';
const FIXTURE_TO_ISO = '2026-03-18T01:00:00.000Z';

function exploreUrl(opts: { rawSql?: string } = {}) {
  const query: Record<string, unknown> = {
    refId: 'A',
    datasource: { type: PLUGIN_ID, uid: DS_UID },
    format: 'table',
  };
  if (opts.rawSql !== undefined) {
    query.rawSql = opts.rawSql;
    query.editorMode = 'code';
  }
  const panes = JSON.stringify({
    a: {
      datasource: DS_UID,
      queries: [query],
      range: { from: FIXTURE_FROM_ISO, to: FIXTURE_TO_ISO },
    },
  });
  return `/explore?orgId=1&schemaVersion=1&panes=${encodeURIComponent(panes)}`;
}

// TODO: remove once @grafana/plugin-e2e exposes body reading natively
async function waitForQueryDataResponseWithBody(explorePage: ExplorePage) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let body: any = null;
  const responsePromise = explorePage.waitForQueryDataResponse(async (r) => {
    if (!r.ok()) {
      return false;
    }
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const b: any = await r.json().catch(() => null);
    if (!Array.isArray(b?.results?.A?.frames)) {
      return false;
    }
    body = b;
    return true;
  });
  return { responsePromise, getBody: () => body };
}

test.describe('Query editor', () => {
  test.describe('rendering', () => {
    test(
      'smoke: should render Builder and Code mode options',
      { tag: '@plugins' },
      async ({ page }) => {
        await page.goto(exploreUrl());
        await expect(page.getByRole('radio', { name: 'Builder' })).toBeVisible();
        await expect(page.getByRole('radio', { name: 'Code' })).toBeVisible();
      }
    );

    test('should show Format selector in both modes', async ({ page }) => {
      await page.goto(exploreUrl());
      await expect(page.getByRole('combobox', { name: 'Format: ' })).toBeVisible();

      await page.getByRole('radio', { name: 'Code' }).click();
      await expect(page.getByRole('radio', { name: 'Code' })).toBeChecked();
      await expect(page.getByRole('combobox', { name: 'Format: ' })).toBeVisible();
    });
  });

  test.describe('Builder mode', () => {
    test('should show Dataset and Table selectors', async ({ page }) => {
      await page.goto(exploreUrl());
      await page.getByRole('radio', { name: 'Builder' }).click();
      await expect(page.getByRole('radio', { name: 'Builder' })).toBeChecked();
      await expect(page.getByRole('combobox', { name: 'Dataset selector' })).toBeVisible();
      await expect(page.getByRole('combobox', { name: 'Table selector' })).toBeVisible();
    });

    test('should show Column selector', async ({ page }) => {
      await page.goto(exploreUrl());
      await page.getByRole('radio', { name: 'Builder' }).click();
      await expect(page.getByRole('radio', { name: 'Builder' })).toBeChecked();
      await expect(page.getByRole('combobox', { name: 'Column' })).toBeVisible();
    });
  });

  test.describe('Code mode', () => {
    test('should show SQL editor', async ({ page }) => {
      await page.goto(exploreUrl({ rawSql: 'SELECT 1' }));
      await expect(page.getByRole('radio', { name: 'Code' })).toBeChecked();
      await expect(page.getByRole('textbox', { name: /editor content/i })).toBeVisible();
    });

    test('should display pre-populated SQL from URL', async ({ page }) => {
      await page.goto(exploreUrl({ rawSql: 'SELECT 1' }));
      await expect(page.getByRole('radio', { name: 'Code' })).toBeChecked();
      await expect(page.getByRole('textbox', { name: /editor content/i })).toHaveValue('SELECT 1');
    });
  });
});

test.describe('Query editor with fixture data', () => {
  test.describe.configure({ mode: 'serial' });

  test('Code mode: should return rows from e2e_metrics', async ({ page, explorePage }) => {
    const sql = 'SELECT time, value, host FROM e2e_metrics ORDER BY time LIMIT 5';
    // Register before goto — query fires on page load when rawSql is encoded in the URL
    const { responsePromise, getBody } = await waitForQueryDataResponseWithBody(explorePage);
    await page.goto(exploreUrl({ rawSql: sql }));
    await responsePromise;
    const body = getBody();
    expect(body?.results?.A?.frames?.length).toBeGreaterThan(0);
    const fields = body.results.A.frames[0].schema.fields.map((f: { name: string }) => f.name);
    expect(fields).toEqual(['time', 'value', 'host']);
  });

  test('Code mode: should return correct row count', async ({ page, explorePage }) => {
    const sql = 'SELECT time, value, host FROM e2e_metrics ORDER BY time';
    const { responsePromise, getBody } = await waitForQueryDataResponseWithBody(explorePage);
    await page.goto(exploreUrl({ rawSql: sql }));
    await responsePromise;
    const body = getBody();
    const values = body.results.A.frames[0].data.values;
    // 17 timestamps × 2 hosts = 34 rows total
    expect(values[0].length).toBe(34);
  });

  test('Code mode: should filter results by host', async ({ page, explorePage }) => {
    const sql = "SELECT time, value, host FROM e2e_metrics WHERE host = 'server-a' ORDER BY time";
    const { responsePromise, getBody } = await waitForQueryDataResponseWithBody(explorePage);
    await page.goto(exploreUrl({ rawSql: sql }));
    await responsePromise;
    const body = getBody();
    const frame = body.results.A.frames[0];
    const hostFieldIndex = frame.schema.fields.findIndex((f: { name: string }) => f.name === 'host');
    const hosts: string[] = frame.data.values[hostFieldIndex];
    expect(hosts.length).toBe(17);
    expect(hosts.every((h) => h === 'server-a')).toBe(true);
  });

  test('Builder mode: should show provisioned dataset in selector', async ({ page }) => {
    await page.goto(exploreUrl());
    await page.getByRole('radio', { name: 'Builder' }).click();
    await expect(page.getByRole('radio', { name: 'Builder' })).toBeChecked();
    // The Dataset selector should show the connected database name
    await expect(
      page.locator('[data-testid="query-editor-row"]').getByText('grafana')
    ).toBeVisible();
  });
});
