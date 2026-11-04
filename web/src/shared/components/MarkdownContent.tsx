import type { ComponentPropsWithoutRef } from 'react'
import Markdown, { type Components, type ExtraProps } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { cn } from '@/shared/lib/utils'

type MarkdownContentProps = {
  markdown: string
  className?: string
}

// Raw HTML is never rendered and react-markdown's default urlTransform drops unsafe protocols; adding rehype-raw or a
// custom urlTransform here would open the description to script injection.
export function MarkdownContent({ markdown, className }: MarkdownContentProps) {
  return (
    <div className={cn('flex flex-col gap-3 text-sm leading-relaxed break-words text-foreground', className)}>
      <Markdown remarkPlugins={remarkPlugins} components={markdownComponents}>
        {markdown}
      </Markdown>
    </div>
  )
}

const remarkPlugins = [remarkGfm]

function withoutNode<Props extends ExtraProps>(props: Props): Omit<Props, 'node'> {
  const elementProps = { ...props }
  delete elementProps.node
  return elementProps
}

function styled<Tag extends keyof React.JSX.IntrinsicElements>(Tag: Tag, className: string) {
  return function MarkdownElement(props: ComponentPropsWithoutRef<Tag> & ExtraProps) {
    const { className: ownClassName, ...rest } = withoutNode(props) as { className?: string }
    const Element = Tag as React.ElementType
    return <Element className={cn(className, ownClassName)} {...rest} />
  }
}

const markdownComponents: Components = {
  h1: styled('h1', 'mt-2 text-xl font-semibold'),
  h2: styled('h2', 'mt-2 text-lg font-semibold'),
  h3: styled('h3', 'mt-1 text-base font-semibold'),
  h4: styled('h4', 'text-sm font-semibold'),
  h5: styled('h5', 'text-sm font-semibold'),
  h6: styled('h6', 'text-sm font-semibold text-muted-foreground'),
  p: styled('p', 'whitespace-pre-wrap'),
  ul: styled('ul', 'flex list-disc flex-col gap-1 pl-5 [&.contains-task-list]:list-none [&.contains-task-list]:pl-1'),
  ol: styled('ol', 'flex list-decimal flex-col gap-1 pl-5'),
  li: styled('li', '[&>ul]:mt-1 [&>ol]:mt-1'),
  input: styled('input', 'mr-2 size-3.5 translate-y-0.5 accent-primary'),
  blockquote: styled('blockquote', 'border-l-2 pl-3 text-muted-foreground'),
  hr: styled('hr', 'border-border'),
  pre: styled('pre', 'overflow-x-auto rounded-md bg-muted p-3 font-mono text-xs [&>code]:bg-transparent [&>code]:p-0'),
  code: styled('code', 'rounded bg-muted px-1 py-0.5 font-mono text-xs'),
  table: styled('table', 'w-full border-collapse text-left'),
  th: styled('th', 'border px-2 py-1 font-medium'),
  td: styled('td', 'border px-2 py-1'),
  a: function MarkdownLink(props) {
    const { className, ...rest } = withoutNode(props)
    return (
      <a
        {...rest}
        className={cn('text-primary underline underline-offset-2', className)}
        target="_blank"
        rel="noopener noreferrer"
      />
    )
  },
}
