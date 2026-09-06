// 应用壳共享 UI 状态（侧栏固定展开/移动端抽屉）。
// 展开/收起由顶栏最左 Logo 触发（产品要求）；菜单点击后自动固定展开。
export const shell = $state<{ sidebarPinned: boolean; mobileNavOpen: boolean }>({
	sidebarPinned: false,
	mobileNavOpen: false
});
