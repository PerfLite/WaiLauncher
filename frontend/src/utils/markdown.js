import {marked} from 'marked'
import {OpenURL} from '../../wailsjs/go/main/App'

const renderer = new marked.Renderer()

renderer.link = function({href, title, text}) {
  const titleAttr = title ? ` title="${title}"` : ''
  return `<a href="${href}"${titleAttr} target="_blank" rel="noopener noreferrer" class="md-link">${text}</a>`
}

marked.setOptions({
  renderer,
  gfm: true,
  breaks: true,
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
