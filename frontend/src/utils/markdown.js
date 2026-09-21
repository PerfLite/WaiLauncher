import {marked} from 'marked'
import {OpenURL} from '../../wailsjs/go/main/App'

marked.use({
  gfm: true,
  breaks: true,
  renderer: {
    link({ href, title, tokens, text }) {
      const inner = (this.parser && tokens && tokens.length)
        ? this.parser.parseInline(tokens)
        : (text || '')
      const titleAttr = title ? ` title="${title}"` : ''
      return `<a href="${href}"${titleAttr} target="_blank" rel="noopener noreferrer" class="md-link">${inner}</a>`
    },
    image({ href, title, text }) {
      const titleAttr = title ? ` title="${title}"` : ''
      const altAttr = text ? ` alt="${text}"` : ''
      return `<img src="${href}"${altAttr}${titleAttr} loading="lazy" class="md-img" />`
    }
  }
})

export function renderMarkdown(content) {
  if (!content) return ''
  try {
    return marked.parse(content)
  } catch (e) {
    console.error('Error parsing markdown:', e)
    return String(content)
  }
}

export function handleMarkdownClick(e) {
  const link = e.target.closest('a')
  if (link && link.href) {
    e.preventDefault()
    try {
      OpenURL(link.href)
    } catch (err) {
      window.open(link.href, '_blank')
    }
  }
}
