import { Component, Fragment } from 'react'
import type { ReactNode } from 'react'
import { Button } from './components/ui'

export class EditorBoundary extends Component<
  { children: ReactNode; close(): Promise<void> },
  { failed: boolean; attempt: number }
> {
  state = { failed: false, attempt: 0 }

  static getDerivedStateFromError() {
    return { failed: true }
  }

  render() {
    if (this.state.failed)
      return (
        <aside className="document-pane" aria-label="Document recovery">
          <div className="document-notice" role="alert">
            <h2>The editor stopped unexpectedly</h2>
            <p>Your saved document and local drafts are still available.</p>
            <Button
              variant="secondary"
              onClick={() =>
                this.setState(({ attempt }) => ({ failed: false, attempt: attempt + 1 }))
              }
            >
              Retry editor
            </Button>
            <Button variant="ghost" onClick={() => void this.props.close()}>
              Return to chat
            </Button>
          </div>
        </aside>
      )
    return <Fragment key={this.state.attempt}>{this.props.children}</Fragment>
  }
}
