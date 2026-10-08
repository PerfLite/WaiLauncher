import { marked } from 'marked'
import { OpenURL } from 'bindings/WaiLauncher/app'

marked.use({
  gfm: true,
  breaks: true,
  renderer: {
    link({ href, title, tokens, text }: any) {
      const inner = (this.parser && tokens && tokens.length)
        ? this.parser.parseInline(tokens)
        : (text || '')
      const titleAttr = title ? ` title="${title}"` : ''
      return `<a href="${href}"${titleAttr} target="_blank" rel="noopener noreferrer" class="md-link">${inner}</a>`
    },
    image({ href, title, text }: any) {
      const titleAttr = title ? ` title="${title}"` : ''
      const altAttr = text ? ` alt="${text}"` : ''
      return `<img src="${href}"${altAttr}${titleAttr} loading="lazy" class="md-img" />`
    }
  }
})

export function renderMarkdown(content: string | undefined | null): string {
  if (!content) return ''
  try {
    return marked.parse(content) as string
  } catch (e) {
    console.error('Error parsing markdown:', e)
    return String(content)
  }
}

export function handleMarkdownClick(e: MouseEvent): void {
  const target = e.target as HTMLElement | null
  const link = target?.closest('a')
  if (link && link.href) {
    e.preventDefault()
    try {
      OpenURL(link.href)
    } catch {
      window.open(link.href, '_blank')
    }
  }
}
