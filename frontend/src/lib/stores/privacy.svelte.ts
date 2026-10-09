/**
 * 隐私模式（隐藏金额）。
 *
 * - hidden：持久开关。写 localStorage 保证首帧即遮蔽，同时同步到后端
 *   User.hide_amounts（跨设备一致，服务端为准）。
 * - revealed：临时显示。只在当前会话有效，刷新/重开恢复遮蔽——
 *   适合在他人瞥一眼时临时展示，不用改动持久设置。
 *
 * formatMoney / formatShortMoney 统一读 masked，实现全站金额遮蔽。
 */
import { http } from '$lib/api/http';

const KEY = 'hz_hide_amounts';

function initialHidden(): boolean {
	if (typeof localStorage === 'undefined') return false;
	try {
		return localStorage.getItem(KEY) === '1';
	} catch {
		return false;
	}
}

let hidden = $state<boolean>(initialHidden());
let revealed = $state<boolean>(false);

function setHidden(v: boolean) {
	hidden = v;
	revealed = false; // 改持久开关后重新收敛，避免「关掉隐私模式却还遮着」
	if (typeof localStorage !== 'undefined') {
		try {
			localStorage.setItem(KEY, v ? '1' : '0');
		} catch {}
	}
	// 展示偏好，失败不影响主流程（下次 checkAuth 会以服务端为准校正）
	if (http.getToken()) {
		http.put('/auth/me', { hide_amounts: v }).catch(() => {});
	}
}

export const privacyStore = {
	get hidden() {
		return hidden;
	},
	get revealed() {
		return revealed;
	},
	/** 当前是否应当遮蔽金额 */
	get masked() {
		return hidden && !revealed;
	},
	/** 侧边栏眼睛按钮：未开启 → 开启遮蔽；已开启 → 临时显示 / 恢复遮蔽 */
	toggle() {
		if (!hidden) setHidden(true);
		else revealed = !revealed;
	},
	/** 设置页开关：改持久设置（含同步服务端） */
	set: setHidden,
	/** 登录后以服务端 User.hide_amounts 为准校正本机状态（同时清除临时显示） */
	hydrate(v: boolean | undefined) {
		revealed = false;
		if (typeof v === 'boolean' && v !== hidden) setHidden(v);
	}
};
