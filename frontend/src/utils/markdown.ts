/**
 * 轻量级 Markdown → HTML 渲染器（无第三方依赖）。
 *
 * 仅覆盖 review-service 周报实际使用的子集：标题（h1-h3）、粗体、无序/有序列表、
 * 分隔线、行内 code、代码块、表格。内容由后端自身生成，但仍对原始文本做 HTML 转义
 * 以防注入。
 *
 * 对于更复杂的 Markdown 场景，应引入 marked + DOMPurify 替换此实现。
 */

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

/** 渲染行内格式（粗体 / 行内 code / 链接）。输入需已转义。 */
function renderInline(escaped: string): string {
  return escaped
    // 行内 code（优先处理，避免内容被其他规则破坏）
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    // 粗体 **text**
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    // 斜体 *text*（避免与粗体冲突，要求两侧非 *）
    .replace(/(^|[^*])\*([^*\n]+)\*(?!\*)/g, '$1<em>$2</em>')
    // 链接 [text](url)
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>')
}

export function renderMarkdown(src: string): string {
  if (!src) return ''
  const lines = src.replace(/\r\n/g, '\n').split('\n')
  const html: string[] = []
  let i = 0

  while (i < lines.length) {
    const line = lines[i]

    // 代码块 ```
    if (/^```/.test(line.trim())) {
      const code: string[] = []
      i++
      while (i < lines.length && !/^```/.test(lines[i].trim())) {
        code.push(lines[i])
        i++
      }
      i++ // 跳过结束 ```
      html.push(`<pre><code>${escapeHtml(code.join('\n'))}</code></pre>`)
      continue
    }

    // 标题
    const headingMatch = /^(#{1,6})\s+(.*)$/.exec(line)
    if (headingMatch) {
      const level = headingMatch[1].length
      html.push(`<h${level}>${renderInline(escapeHtml(headingMatch[2]))}</h${level}>`)
      i++
      continue
    }

    // 分隔线
    if (/^(-{3,}|\*{3,}|_{3,})\s*$/.test(line)) {
      html.push('<hr />')
      i++
      continue
    }

    // 表格（含分隔行 |---|）
    if (line.includes('|') && i + 1 < lines.length && /^\s*\|?[\s:|-]+\|?\s*$/.test(lines[i + 1])) {
      const headerCells = line.split('|').map((c) => c.trim()).filter((c) => c !== '')
      i += 2 // 跳过分隔行
      const rows: string[][] = []
      while (i < lines.length && lines[i].includes('|') && lines[i].trim() !== '') {
        rows.push(lines[i].split('|').map((c) => c.trim()).filter((c) => c !== ''))
        i++
      }
      const thead = `<thead><tr>${headerCells.map((c) => `<th>${renderInline(escapeHtml(c))}</th>`).join('')}</tr></thead>`
      const tbody = `<tbody>${rows.map((r) => `<tr>${r.map((c) => `<td>${renderInline(escapeHtml(c))}</td>`).join('')}</tr>`).join('')}</tbody>`
      html.push(`<table>${thead}${tbody}</table>`)
      continue
    }

    // 无序列表
    if (/^\s*[-*+]\s+/.test(line)) {
      const items: string[] = []
      while (i < lines.length && /^\s*[-*+]\s+/.test(lines[i])) {
        items.push(lines[i].replace(/^\s*[-*+]\s+/, ''))
        i++
      }
      html.push(`<ul>${items.map((it) => `<li>${renderInline(escapeHtml(it))}</li>`).join('')}</ul>`)
      continue
    }

    // 有序列表
    if (/^\s*\d+\.\s+/.test(line)) {
      const items: string[] = []
      while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) {
        items.push(lines[i].replace(/^\s*\d+\.\s+/, ''))
        i++
      }
      html.push(`<ol>${items.map((it) => `<li>${renderInline(escapeHtml(it))}</li>`).join('')}</ol>`)
      continue
    }

    // 引用
    if (/^>\s+/.test(line)) {
      const quote: string[] = []
      while (i < lines.length && /^>\s+/.test(lines[i])) {
        quote.push(lines[i].replace(/^>\s+/, ''))
        i++
      }
      html.push(`<blockquote>${renderInline(escapeHtml(quote.join(' ')))}</blockquote>`)
      continue
    }

    // 空行
    if (line.trim() === '') {
      i++
      continue
    }

    // 普通段落（连续非空行合并）
    const para: string[] = []
    while (
      i < lines.length &&
      lines[i].trim() !== '' &&
      !/^(#{1,6})\s/.test(lines[i]) &&
      !/^\s*[-*+]\s/.test(lines[i]) &&
      !/^\s*\d+\.\s/.test(lines[i]) &&
      !/^```/.test(lines[i].trim()) &&
      !/^>\s/.test(lines[i]) &&
      !/^(-{3,}|\*{3,}|_{3,})\s*$/.test(lines[i])
    ) {
      para.push(lines[i])
      i++
    }
    html.push(`<p>${renderInline(escapeHtml(para.join(' ')))}</p>`)
  }

  return html.join('\n')
}
