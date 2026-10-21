import { createMemoryHistory } from '@tanstack/react-router'
import { render } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { App } from '../app/App'

export function renderApp(path: string) {
  const user = userEvent.setup()
  const history = createMemoryHistory({ initialEntries: [path] })
  return { user, history, ...render(<App history={history} />) }
}
