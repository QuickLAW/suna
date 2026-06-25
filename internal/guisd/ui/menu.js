/* menu.js — VSCode 风格顶部菜单栏。
   设计原则：
   - 菜单结构（哪些菜单 / 哪些项）由本文件静态定义；动作（点击要做什么）由外部注入。
   - 单实例：工厂 createMenuBar 返回菜单实例，整个 GUI 共用一个。
   - 键盘可用：trigger 支持 Enter / 方向键切换；dropdown 内方向键循环、Esc 关闭。
   - 同一时刻最多一个 dropdown 展开；点击空白处自动关闭。
   - 菜单项 disabled 状态由 disabled(ctx) 实时计算，context 由外部传入。
   - 动作通过 actions[actionId]() 调用；调用前关闭 dropdown，失败不抛出。 */

import { el } from './core.js';

// MENUS 描述所有菜单与菜单项。type: 'sep' 是分隔线。
// action 字段是动作 id，对应外部注入的 actions[action]。
// disabled(ctx) 返回 true 时菜单项置灰且不可点击。
const MENUS = [
  {
    id: 'file',
    label: '文件',
    items: [
      { id: 'quick-open', label: '快速打开文件…', shortcut: 'Ctrl+P', action: 'open-quickopen' },
      { id: 'global-search', label: '全局搜索', shortcut: 'Ctrl+Shift+F', action: 'open-search' },
      { type: 'sep' },
      { id: 'close-tab', label: '关闭当前标签页', shortcut: 'Ctrl+W', action: 'close-tab', disabled: (ctx) => !ctx.hasActiveFile },
      { type: 'sep' },
      { id: 'go-home', label: '回到工作目录', action: 'go-home' },
    ],
  },
  {
    id: 'view',
    label: '视图',
    items: [
      { id: 'toggle-chat', label: '切换聊天面板', action: 'toggle-chat' },
      { type: 'sep' },
      { id: 'config', label: '设置…', action: 'open-config' },
      { id: 'help', label: '帮助…', shortcut: '?', action: 'open-help' },
    ],
  },
  {
    id: 'terminal',
    label: '终端',
    items: [
      { id: 'new-term', label: '新建终端', action: 'new-term' },
      { id: 'clear-term', label: '清空当前终端', action: 'clear-term' },
    ],
  },
  {
    id: 'help',
    label: '帮助',
    items: [
      { id: 'open-help', label: '查看帮助', action: 'open-help' },
      { type: 'sep' },
      { id: 'about', label: '关于 Suna', action: 'about' },
    ],
  },
];

// 导出菜单结构（只读），方便测试 / 调试时枚举所有 action id。
export const MENU_ACTIONS = MENUS.flatMap(m => m.items.filter(i => i.action).map(i => i.action));

export function createMenuBar(deps) {
  const { actions, getContext } = deps;
  let container = null;
  let openMenuId = null;
  let bound = false;

  // closeAll 关闭所有 dropdown，并清掉 .open 状态。
  function closeAll() {
    if (!container) return;
    container.querySelectorAll('.menu-trigger.open').forEach(t => t.classList.remove('open'));
    container.querySelectorAll('.menu-dropdown').forEach(d => d.remove());
    openMenuId = null;
  }

  // openMenu 切换指定菜单的 dropdown（再点同一菜单则关闭）。
  // dropdown 渲染时立刻调用 getContext()，让 disabled 拿到最新状态。
  function openMenu(menuId) {
    if (!container) return;
    if (openMenuId === menuId) { closeAll(); return; }
    closeAll();
    const trigger = container.querySelector('.menu-trigger[data-menu="' + menuId + '"]');
    const menu = MENUS.find(m => m.id === menuId);
    if (!trigger || !menu) return;
    trigger.classList.add('open');
    const ctx = (typeof getContext === 'function') ? (getContext() || {}) : {};
    const drop = buildDropdown(menu, ctx);
    container.append(drop);
    // 越界检测：默认向左贴齐 trigger 左边缘；如右侧越界则改为右贴齐。
    requestAnimationFrame(() => {
      const r = drop.getBoundingClientRect();
      if (r.right > window.innerWidth - 4) {
        drop.style.left = 'auto';
        drop.style.right = '0';
        drop.style.transform = 'none';
      }
    });
    openMenuId = menuId;
  }

  // buildDropdown 构造一个菜单的 dropdown DOM。
  // 菜单项被点击后：先关 dropdown，再执行 action；action 不存在时静默忽略。
  function buildDropdown(menu, ctx) {
    const drop = el('div', { class: 'menu-dropdown', dataset: { menu: menu.id } });
    menu.items.forEach((item) => {
      if (item.type === 'sep') {
        drop.append(el('div', { class: 'menu-sep' }));
        return;
      }
      const isDisabled = item.disabled ? !!item.disabled(ctx) : false;
      const row = el('div', {
        class: 'menu-item' + (isDisabled ? ' disabled' : ''),
        dataset: { action: item.action || '' },
        tabindex: '-1',
      }, [
        el('span', { class: 'menu-item-label', text: item.label }),
        item.shortcut ? el('span', { class: 'menu-item-shortcut', text: item.shortcut }) : null,
      ]);
      if (!isDisabled) {
        row.addEventListener('click', (e) => {
          e.stopPropagation();
          const act = item.action;
          closeAll();
          if (act && typeof actions[act] === 'function') {
            try { actions[act](); } catch (err) { console.error('menu action failed', act, err); }
          }
        });
      }
      drop.append(row);
    });
    return drop;
  }

  // buildTrigger 构造菜单条上的 trigger 按钮。
  // 键盘：Enter / Space 打开；方向键在 trigger 之间循环；Esc 关闭。
  function buildTrigger(menu) {
    const btn = el('button', {
      class: 'menu-trigger',
      type: 'button',
      dataset: { menu: menu.id },
      tabindex: '0',
      title: menu.label,
    });
    btn.textContent = menu.label;
    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      openMenu(menu.id);
    });
    btn.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        openMenu(menu.id);
        focusFirst();
      } else if (e.key === 'ArrowDown') {
        e.preventDefault();
        openMenu(menu.id);
        focusFirst();
      } else if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
        e.preventDefault();
        moveTrigger(e.key === 'ArrowRight' ? 1 : -1);
      } else if (e.key === 'Escape') {
        e.preventDefault();
        closeAll();
      }
    });
    return btn;
  }

  // focusFirst 把焦点移到当前 dropdown 的第一个可点击菜单项。
  function focusFirst() {
    if (!container) return;
    const item = container.querySelector('.menu-dropdown .menu-item:not(.disabled)');
    if (item) item.focus();
  }

  // moveTrigger 焦点在 trigger 之间左右循环。
  function moveTrigger(dir) {
    if (!container) return;
    const triggers = [...container.querySelectorAll('.menu-trigger')];
    if (!triggers.length) return;
    const idx = triggers.findIndex(t => t === document.activeElement);
    const next = triggers[((idx < 0 ? 0 : idx) + dir + triggers.length) % triggers.length];
    if (next) next.focus();
  }

  // render 把所有 trigger 渲染到 container。
  function render() {
    if (!container) return;
    container.classList.add('menu-bar');
    MENUS.forEach(m => container.append(buildTrigger(m)));
  }

  // mount 挂载到指定容器。重复挂载会先清空。
  function mount(root) {
    if (container) unmount();
    container = root;
    root.innerHTML = '';
    render();
    bindGlobalEvents();
  }

  // unmount 卸载菜单栏。
  function unmount() {
    closeAll();
    if (container) container.innerHTML = '';
    container = null;
  }

  // bindGlobalEvents 全局关闭 / 键盘导航。同一实例只绑一次。
  function bindGlobalEvents() {
    if (bound) return;
    bound = true;
    document.addEventListener('click', (e) => {
      if (!container) return;
      if (container.contains(e.target)) return;
      closeAll();
    });
    document.addEventListener('keydown', (e) => {
      if (!openMenuId) return;
      if (e.key === 'Escape') {
        e.preventDefault();
        closeAll();
        return;
      }
      if (!container) return;
      const drop = container.querySelector('.menu-dropdown');
      if (!drop) return;
      const items = [...drop.querySelectorAll('.menu-item:not(.disabled)')];
      if (!items.length) return;
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault();
        const idx = items.findIndex(it => it === document.activeElement);
        const dir = e.key === 'ArrowDown' ? 1 : -1;
        const next = items[((idx < 0 ? 0 : idx) + dir + items.length) % items.length];
        if (next) next.focus();
      } else if (e.key === 'Home' || e.key === 'End') {
        e.preventDefault();
        const target = e.key === 'Home' ? items[0] : items[items.length - 1];
        if (target) target.focus();
      } else if (e.key === 'Tab') {
        e.preventDefault();
        closeAll();
      }
    });
  }

  return { mount, unmount, openMenu, closeAll };
}
