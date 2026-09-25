import { test, expect } from '@playwright/test';

// 桥式网络主流程：页面预填的示例即平衡电桥（R5 为桥臂）。
// 提交后应看到精确的分数结果：R_eq = 1 Ω，桥臂电流精确为 0。
test('桥式网络：提交后展示精确分数结果', async ({ page }) => {
	await page.goto('/');

	// 预填即为惠斯通电桥：A→D，R1..R5 各 1000 毫欧
	await expect(page.locator('input[name="nodes"]')).toHaveValue('A,B,C,D');
	await page.getByRole('button', { name: '求解' }).click();

	// 等效电阻精确为 1 Ω（分数而非浮点）
	await expect(page.getByTestId('eq-resistance')).toHaveText('1 Ω');

	// SVG 渲染出 4 个节点
	await expect(page.locator('svg circle.node')).toHaveCount(4);

	// 平衡电桥的桥臂电流精确为 0，B、C 等电位 1/2 V
	await expect(page.getByTestId('current-R5')).toHaveText('0 A');
	await expect(page.getByTestId('potential-B')).toHaveText('1/2 V');
	await expect(page.getByTestId('potential-C')).toHaveText('1/2 V');
	await expect(page.getByTestId('current-R1')).toHaveText('1/2 A');
});
