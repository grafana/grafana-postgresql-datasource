import { expect, test } from '@grafana/plugin-e2e';
import { type Locator, type Page } from '@playwright/test';

// Grafana 13 migrated query editor row selectors from aria-label to data-testid
// (grafana/grafana#121784). This helper matches both so tests work across versions.
function getQueryEditorRow(page: Page, refId: string): Locator {
  return page
    .locator('[data-testid="data-testid Query editor row"], [aria-label="Query editor row"]')
    .filter({
      has: page.locator(
        `[data-testid="data-testid Query editor row title ${refId}"], [aria-label="Query editor row title ${refId}"]`
      ),
    });
}

test.describe('Query editor', () => {
  test.beforeEach(async ({ panelEditPage, readProvisionedDataSource }) => {
    const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
    await panelEditPage.datasource.set(ds.name);
  });

  test(
    'smoke: should render query editor',
    { tag: '@plugins' },
    async ({ page }) => {
      const row = getQueryEditorRow(page, 'A');

      // @grafana/sql renders Builder / Code mode toggle in the query header
      await expect(row.getByRole('radio', { name: 'Builder' })).toBeVisible();
      await expect(row.getByRole('radio', { name: 'Code' })).toBeVisible();
    }
  );

  test('should switch to Code mode', async ({ page }) => {
    const row = getQueryEditorRow(page, 'A');

    await row.getByRole('radio', { name: 'Code' }).click();

    // The SQL raw editor uses @grafana/plugin-ui SQLEditor → @grafana/ui CodeEditor → Monaco.
    // Monaco renders a .monaco-editor container (not CodeMirror's .cm-editor).
    await expect(row.locator('.monaco-editor')).toBeVisible();
  });
});
