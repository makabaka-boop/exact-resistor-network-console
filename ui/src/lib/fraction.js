/**
 * 把后端返回的约分分数 {num, den} 格式化为字符串。
 * 只使用精确的分子/分母，绝不做浮点近似——
 * 很小但非零的支路电流必须原样显示。
 */
export function fmt(f) {
	return f.den === '1' ? f.num : `${f.num}/${f.den}`;
}
