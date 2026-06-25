/* highlight.js — 超轻量语法高亮，零依赖。
   原理：textarea 透明文字叠在 pre.highlight 之上，highlight 层用 innerHTML 渲染着色后的 HTML。
   支持 Go / JS / TS / HTML / CSS / JSON / Markdown / Python / Shell / TOML / YAML。 */

// 规则集：每个语言一组 [正则, className] 按优先级匹配。
// 所有正则必须带 g 标志（replace 全局替换）。
const rules = {
  go: [
    [/\/\/.*$/gm, 'cm'],                              // 行注释
    [/\/\*[\s\S]*?\*\//g, 'cm'],                      // 块注释
    [/"(?:\\.|[^"\\])*"/g, 'st'],                     // 字符串
    [/'(?:\\.|[^'\\])*'/g, 'st'],                     // rune
    [/\b(?:package|import|func|var|const|type|struct|interface|map|chan|go|defer|return|if|else|for|range|switch|case|default|break|continue|select|fallthrough|goto)\b/g, 'kw'],
    [/\b(?:bool|byte|complex64|complex128|float32|float64|int|int8|int16|int32|int64|uint|uint8|uint16|uint32|uint64|uintptr|string|rune|error|any|nil|true|false)\b/g, 'ty'],
    [/\b\d+(?:\.\d+)?\b/g, 'nu'],                     // 数字
    [/\b[A-Z]\w*\b/g, 'ty'],                          // 导出标识符
  ],
  js: [
    [/\/\/.*$/gm, 'cm'],
    [/\/\*[\s\S]*?\*\//g, 'cm'],
    [/"(?:\\.|[^"\\])*"/g, 'st'],
    [/'(?:\\.|[^'\\])*'/g, 'st'],
    [/`(?:\\.|[^`\\])*`/g, 'st'],
    [/\b(?:const|let|var|function|return|if|else|for|while|do|switch|case|break|continue|new|delete|typeof|instanceof|in|of|class|extends|super|this|import|export|from|default|try|catch|finally|throw|async|await|yield|void|null|undefined|true|false)\b/g, 'kw'],
    [/\b(?:number|string|boolean|object|symbol|bigint)\b/g, 'ty'],
    [/\b\d+(?:\.\d+)?\b/g, 'nu'],
  ],
  ts: [
    [/\/\/.*$/gm, 'cm'],
    [/\/\*[\s\S]*?\*\//g, 'cm'],
    [/"(?:\\.|[^"\\])*"/g, 'st'],
    [/'(?:\\.|[^'\\])*'/g, 'st'],
    [/`(?:\\.|[^`\\])*`/g, 'st'],
    [/\b(?:const|let|var|function|return|if|else|for|while|do|switch|case|break|continue|new|delete|typeof|instanceof|in|of|class|extends|super|this|import|export|from|default|try|catch|finally|throw|async|await|yield|void|null|undefined|true|false|type|interface|enum|namespace|readonly|public|private|protected|abstract|implements|as|is|keyof|never|unknown|any)\b/g, 'kw'],
    [/\b(?:number|string|boolean|object|symbol|bigint)\b/g, 'ty'],
    [/\b\d+(?:\.\d+)?\b/g, 'nu'],
  ],
  html: [
    [/<!--[\s\S]*?-->/g, 'cm'],
    [/<\/?\w+/g, 'kw'],
    [/[\/]?>/g, 'ty'],
    [/[\w-]+(?==)/g, 'at'],
    [/"[^"]*"/g, 'st'],
    [/'[^']*'/g, 'st'],
  ],
  css: [
    [/\/\*[\s\S]*?\*\//g, 'cm'],
    [/[#.][\w-]+/g, 'kw'],
    [/[\w-]+(?=\s*:)/g, 'at'],
    [/:[^;{}]+;/g, 'st'],
    [/[{};]/g, 'ty'],
  ],
  json: [
    [/"[^"]*"(?=\s*:)/g, 'at'],   // key
    [/"[^"]*"/g, 'st'],            // string value
    [/\b(?:true|false|null)\b/g, 'kw'],
    [/\b-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?\b/g, 'nu'],
  ],
  md: [
    [/^#{1,6}\s.+$/gm, 'kw'],      // 标题
    [/```[\s\S]*?```/g, 'cm'],      // 代码块
    [/"[^"]*"/g, 'st'],
    [/\*\*[^*]+\*\*/g, 'st'],
    [/\bhttps?:\/\/\S+/g, 'st'],
  ],
  py: [
    [/#.*$/gm, 'cm'],
    [/"""[\s\S]*?"""/g, 'cm'],
    [/"(?:\\.|[^"\\])*"/g, 'st'],
    [/'(?:\\.|[^'\\])*'/g, 'st'],
    [/\b(?:def|class|return|if|elif|else|for|while|break|continue|pass|import|from|as|try|except|finally|raise|with|lambda|yield|global|nonlocal|assert|del|in|is|not|and|or|None|True|False|self)\b/g, 'kw'],
    [/\b\d+(?:\.\d+)?\b/g, 'nu'],
  ],
  sh: [
    [/#.*$/gm, 'cm'],
    [/"[^"]*"/g, 'st'],
    [/'[^']*'/g, 'st'],
    [/\b(?:if|then|else|fi|for|do|done|while|case|esac|in|function|return|local|export|echo|printf|read|cd|pwd|ls|mkdir|rm|cp|mv|cat|grep|sed|awk|find|chmod|chown|sudo|apt|brew)\b/g, 'kw'],
  ],
  yaml: [
    [/#.*$/gm, 'cm'],
    [/"[^"]*"/g, 'st'],
    [/'[^']*'/g, 'st'],
    [/^[\s]*[\w-]+(?=:)/gm, 'at'],   // key
    [/\b(?:true|false|null|yes|no)\b/gi, 'kw'],
    [/\b\d+(?:\.\d+)?\b/g, 'nu'],
  ],
  toml: [
    [/#.*$/gm, 'cm'],
    [/"[^"]*"/g, 'st'],
    [/'[^']*'/g, 'st'],
    [/^\[.*\]$/gm, 'kw'],
    [/[\w-]+(?==)/g, 'at'],
    [/\b\d+(?:\.\d+)?\b/g, 'nu'],
    [/\b(?:true|false)\b/g, 'kw'],
  ],
  default: [],
};

// langForFile 根据文件扩展名返回高亮语言。
export function langForFile(name) {
  const ext = name.split('.').pop().toLowerCase();
  const map = {
    go: 'go', js: 'js', mjs: 'js', jsx: 'js', ts: 'ts', tsx: 'ts',
    html: 'html', htm: 'html', css: 'css', json: 'json', md: 'md', markdown: 'md',
    py: 'py', sh: 'sh', bash: 'sh', zsh: 'sh', toml: 'toml',
    yml: 'yaml', yaml: 'yaml',
  };
  return map[ext] || 'default';
}

// highlight 将源码文本转为带语法高亮的 HTML。
// 先转义 HTML，再用占位符提取已匹配区域避免重叠。
export function highlight(code, lang) {
  const langRules = rules[lang] || rules.default;
  if (langRules.length === 0) {
    return escapeHtml(code);
  }

  // 转义 HTML
  let html = escapeHtml(code);
  // 用占位符提取已高亮区域，避免后续规则重复匹配。
  // 占位符索引编码为字母（a-p），避免被 \d+ 等数字规则误匹配。
  const placeholders = [];
  const encodeIdx = (n) => {
    // 将数字编码为字母序列（base 16，用 a-p）
    let s = '';
    do {
      s = String.fromCharCode(97 + (n % 16)) + s;
      n = Math.floor(n / 16);
    } while (n > 0);
    return s;
  };
  const PH = (cls, text) => {
    const i = placeholders.length;
    placeholders.push(`<span class="hl-${cls}">${text}</span>`);
    return `\x01${encodeIdx(i)}\x01`;
  };

  for (const [pattern, cls] of langRules) {
    html = html.replace(pattern, (match) => PH(cls, match));
  }

  // 还原所有占位符（解码字母索引回数字）
  html = html.replace(/\x01([a-p]+)\x01/g, (_, s) => {
    let n = 0;
    for (const ch of s) n = n * 16 + (ch.charCodeAt(0) - 97);
    return placeholders[n];
  });

  // 确保末尾换行（textarea 自动补行，highlight 层需要同步）
  if (html.endsWith('\n')) {
    html += ' ';
  }
  return html;
}

// escapeHtml 转义 HTML 特殊字符（纯字符串版）。
// 与 core.js 的 escapeHtml（DOM 方式）行为一致但实现不同，
// 仅覆盖 & < > 三个字符，对语法高亮场景足够。
// 不从 core.js 导入以避免循环依赖（highlight.js 是无依赖叶子模块）。
function escapeHtml(text) {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}
