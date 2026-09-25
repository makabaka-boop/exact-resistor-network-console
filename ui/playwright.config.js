import { defineConfig } from '@playwright/test';

// 默认通过 docker compose 拉起完整栈（ui + circuit 真实 HTTP 联调）；
// 若已自行启动服务，可设 PLAYWRIGHT_BASE_URL 跳过。
export default defineConfig({
	testDir: './tests',
	timeout: 30_000,
	retries: 0,
	use: {
		baseURL: process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:3000'
	},
	webServer: process.env.PLAYWRIGHT_BASE_URL
		? undefined
		: {
				command: 'docker compose -f ../compose.yml up --build',
				url: 'http://localhost:3000',
				reuseExistingServer: true,
				timeout: 300_000
			}
});
