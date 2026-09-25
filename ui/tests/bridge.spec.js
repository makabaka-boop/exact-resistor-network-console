import { test, expect } from '@playwright/test';

// Main browser flow: a balanced Wheatstone bridge — a network that cannot be
// reduced by series/parallel rules. The exact solver must show an equivalent
// of 1000 mΩ and a mid-branch current of exactly 0 A (no floating-point dust).
test('bridge network main flow', async ({ page }) => {
  await page.goto('/');

  await page.getByRole('button', { name: 'Bridge' }).click();
  await page.getByRole('button', { name: 'Solve' }).click();

  // equivalent resistance from the exact rational solution
  await expect(page.getByTestId('equivalent')).toHaveText('1000 mΩ');

  // node potentials rendered in the table
  await expect(page.getByTestId('potential-A')).toHaveText('1000 mV');
  await expect(page.getByTestId('potential-B')).toHaveText('500 mV');
  await expect(page.getByTestId('potential-D')).toHaveText('0 mV');

  // balanced bridge: the middle branch carries exactly zero current
  await expect(page.getByTestId('current-R5')).toHaveText('0 A');
  await expect(page.getByTestId('current-R1')).toHaveText('1/2 A');

  // the SVG graph renders nodes and edge labels from the same response
  const graph = page.getByTestId('graph');
  await expect(graph).toBeVisible();
  await expect(graph.locator('circle')).toHaveCount(4);
  await expect(graph.getByText('R5 · 0 A')).toBeVisible();
});
