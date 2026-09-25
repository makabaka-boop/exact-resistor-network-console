import { fail } from '@sveltejs/kit';

// Compose 中由环境变量指向 circuit 容器（http://circuit:8080）。
const CIRCUIT_URL = process.env.CIRCUIT_URL ?? 'http://localhost:8080';

/** @type {import('./$types').Actions} */
export const actions = {
	default: async ({ request }) => {
		const fd = await request.formData();
		const nodes = String(fd.get('nodes') ?? '')
			.split(/[\s,]+/)
			.filter(Boolean);
		const source = String(fd.get('source') ?? '').trim();
		const sink = String(fd.get('sink') ?? '').trim();

		const ids = fd.getAll('rid').map((s) => String(s).trim());
		const froms = fd.getAll('rfrom').map((s) => String(s).trim());
		const tos = fd.getAll('rto').map((s) => String(s).trim());
		const mohs = fd.getAll('rmoh').map((s) => String(s).trim());

		const resistors = [];
		for (let i = 0; i < ids.length; i++) {
			const id = ids[i];
			const from = froms[i] ?? '';
			const to = tos[i] ?? '';
			const moh = mohs[i] ?? '';
			if (!id && !from && !to && !moh) continue; // 整行留空则忽略
			if (!/^[1-9]\d{0,14}$/.test(moh)) {
				return fail(400, { error: `电阻 ${id || `第 ${i + 1} 行`} 的毫欧值必须是正整数` });
			}
			resistors.push({ id, from, to, milliohms: Number(moh) });
		}

		let res;
		let body;
		try {
			res = await fetch(`${CIRCUIT_URL}/api/solve`, {
				method: 'POST',
				headers: { 'content-type': 'application/json' },
				body: JSON.stringify({ nodes, resistors, source, sink })
			});
			body = await res.json();
		} catch {
			return fail(502, { error: `无法连接求解服务（${CIRCUIT_URL}）` });
		}
		if (!res.ok) {
			return fail(res.status, { error: body.error ?? '求解服务返回错误', components: body.components });
		}
		return { result: body };
	}
};
