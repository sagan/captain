import { Fragment, useMemo } from 'react'
import type { HTMLAttributes } from 'react'
import { parseMarkdown, sanitizeMarkdownUrl } from '@shared/utils/markdown'
import '@/scoped/markdown-renderer.css'
import { scope } from '@/scoped/scope'

const s = scope('markdown-renderer')

export default function MarkdownRenderer({ content, ...attrs }: { content: string } & HTMLAttributes<HTMLSpanElement>) {
  const tokens = useMemo(() => parseMarkdown(content), [content])
  return (
    <span {...attrs} {...s} className={attrs.className ? `markdown-content ${attrs.className}` : 'markdown-content'}>
      {tokens.map((token, index) => (
        <Fragment key={index}>
          {token.type === 'image'
            ? (
                <img
                  {...s}
                  src={sanitizeMarkdownUrl(token.url, 'image')}
                  alt={token.alt}
                  loading="lazy"
                  className="align-middle h-auto max-w-full inline-block rounded"
                  style={{ maxHeight: '200px' }}
                />
              )
            : token.type === 'link'
              ? (
                  <a
                    {...s}
                    href={sanitizeMarkdownUrl(token.url, 'link')}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary underline-offset-4 hover:underline"
                  >
                    {token.content}
                  </a>
                )
              : token.type === 'bold'
                ? <strong {...s}>{token.content}</strong>
                : token.type === 'italic'
                  ? <em {...s}>{token.content}</em>
                  : token.type === 'code'
                    ? <code {...s} className="px-1.5 py-0.5 rounded bg-muted text-foreground text-[0.85em] font-mono">{token.content}</code>
                    : token.type === 'br'
                      ? <br {...s} />
                      : token.type === 'text'
                        ? <span {...s}>{token.content}</span>
                        : null}
        </Fragment>
      ))}
    </span>
  )
}
