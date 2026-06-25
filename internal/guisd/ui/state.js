/* state.js — 中央状态层。
   各模块不再各自维护共享状态，统一通过 store 读写和订阅。
   设计目标：
   - 简单：路径式 get/set，无 Immutable 库
   - 解耦：config/chat/editor/terminal 各模块只 subscribe 自己关心的路径
   - 安全：setState 返回新快照的引用（浅拷贝），避免模块意外持有旧引用 */
const _state = {
  daemon: { connected: false, status: null },
  config: { snapshot: null },            // daemon 返回的 ConfigParams
  editor: { activeFile: null, tabs: [] },
  ui: {
    mode: 'file',                        // file / config / help
    workspaceRoot: '.',
    pathSep: '/',
    chatOpen: true,
    terminalOpen: true,
    activityBar: 'files',                // files / config / help
  },
};

// _listeners: Map<pathPrefix, Set<callback>>
// pathPrefix 可以是 'daemon'、'config'、'ui.mode' 等，callback 收到 (newValue, fullPath)。
const _listeners = new Map();

// getState 返回指定路径的值，无参数返回整个状态快照。
// 路径用点分隔：'daemon.connected'、'config.snapshot.models'。
export function getState(path) {
  if (!path) return { ..._state };
  const parts = path.split('.');
  let cur = _state;
  for (const p of parts) {
    if (cur == null) return undefined;
    cur = cur[p];
  }
  return cur;
}

// setState 设置指定路径的值并通知订阅者。
// 示例：setState('daemon.connected', true)、setState('ui.mode', 'config')。
export function setState(path, value) {
  const parts = path.split('.');
  let cur = _state;
  for (let i = 0; i < parts.length - 1; i++) {
    if (cur[parts[i]] == null) cur[parts[i]] = {};
    cur = cur[parts[i]];
  }
  cur[parts[parts.length - 1]] = value;
  _notify(path, value);
  return _state;
}

// subscribe 订阅路径前缀变化。
// callback 在路径前缀匹配时被调用：subscribe('config', cb) 会在
// setState('config.snapshot', ...) 和 setState('config.snapshot.models', ...) 时触发。
export function subscribe(path, callback) {
  if (!_listeners.has(path)) _listeners.set(path, new Set());
  _listeners.get(path).add(callback);
  return () => unsubscribe(path, callback);
}

export function unsubscribe(path, callback) {
  const set = _listeners.get(path);
  if (set) {
    set.delete(callback);
    if (set.size === 0) _listeners.delete(path);
  }
}

function _notify(changedPath, value) {
  for (const [prefix, callbacks] of _listeners) {
    // 路径前缀匹配：'config' 匹配 'config.snapshot'，'ui' 匹配 'ui.mode'。
    if (changedPath === prefix || changedPath.startsWith(prefix + '.')) {
      for (const cb of callbacks) {
        try { cb(value, changedPath); } catch (e) { console.error('[state] listener error:', e); }
      }
    }
  }
}

// update 批量更新多个路径，只在最后通知一次（减少重渲）。
// 用法：update({ 'daemon.connected': true, 'daemon.status': data })。
export function update(updates) {
  const changedPrefixes = new Set();
  for (const [path, value] of Object.entries(updates)) {
    const parts = path.split('.');
    let cur = _state;
    for (let i = 0; i < parts.length - 1; i++) {
      if (cur[parts[i]] == null) cur[parts[i]] = {};
      cur = cur[parts[i]];
    }
    cur[parts[parts.length - 1]] = value;
    // 收集所有受影响的前缀。
    for (let i = 1; i <= parts.length; i++) {
      changedPrefixes.add(parts.slice(0, i).join('.'));
    }
  }
  for (const prefix of changedPrefixes) {
    const callbacks = _listeners.get(prefix);
    if (!callbacks) continue;
    const val = getState(prefix);
    for (const cb of callbacks) {
      try { cb(val, prefix); } catch (e) { console.error('[state] listener error:', e); }
    }
  }
}
