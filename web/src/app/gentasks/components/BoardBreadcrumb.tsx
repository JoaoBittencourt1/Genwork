import { Link } from '@tanstack/react-router'
import { Fragment } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/shared/ui/breadcrumb'
import type { BoardSummary } from '../domain/types'

type BoardBreadcrumbProps = {
  board?: BoardSummary
  current?: string
}

// BoardBreadcrumb reads "Tasks › Board › Current"; the last item is the page title.
export function BoardBreadcrumb({ board, current }: BoardBreadcrumbProps) {
  const { t } = useTranslation()
  const trail = [
    { key: 'boards', label: t('app.nav.tasks'), link: <Link to="/tasks">{t('app.nav.tasks')}</Link> },
    ...(board
      ? [
          {
            key: 'board',
            label: board.name,
            link: (
              <Link to="/tasks/$boardToken" params={{ boardToken: board.token }}>
                {board.name}
              </Link>
            ),
          },
        ]
      : []),
    ...(current ? [{ key: 'current', label: current, link: null }] : []),
  ]
  const last = trail.length - 1

  return (
    <Breadcrumb className="min-w-0">
      <BreadcrumbList className="flex-nowrap">
        {trail.map((item, index) => (
          <Fragment key={item.key}>
            {index > 0 ? <BreadcrumbSeparator /> : null}
            <BreadcrumbItem className="min-w-0">
              {index === last ? (
                <h1 className="min-w-0 truncate">
                  <BreadcrumbPage className="font-medium">{item.label}</BreadcrumbPage>
                </h1>
              ) : (
                <BreadcrumbLink asChild>{item.link}</BreadcrumbLink>
              )}
            </BreadcrumbItem>
          </Fragment>
        ))}
      </BreadcrumbList>
    </Breadcrumb>
  )
}
