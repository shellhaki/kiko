import { useState } from 'react'
import { Excalidraw, exportToSvg } from '@excalidraw/excalidraw'
import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'
import '@excalidraw/excalidraw/index.css'
import { desktop } from './api'
import { Button } from './components/ui'
import { sceneElements, staticSvg } from './diagram-scene'

Object.assign(window, { EXCALIDRAW_ASSET_PATH: '/excalidraw/' })

export type DiagramResult = { scene: string; svg: string; caption: string }

export default function DiagramCanvas({
  scene,
  caption,
  onApply,
}: {
  scene: string
  caption: string
  onApply(result: DiagramResult): void
}) {
  const [api, setApi] = useState<ExcalidrawImperativeAPI | null>(null)
  const [text, setText] = useState(caption)
  const [error, setError] = useState('')
  const theme = document.documentElement.dataset.theme
  const dark =
    theme === 'dark' || (theme === 'system' && !matchMedia('(prefers-color-scheme: light)').matches)
  const apply = async () => {
    const elements = api?.getSceneElements() ?? []
    if (!elements.length) return setError('Draw something before applying.')
    try {
      const drawing = await exportToSvg({
        elements,
        appState: { exportBackground: true, viewBackgroundColor: '#ffffff' },
        files: null,
        skipInliningFonts: true,
        exportPadding: 16,
      })
      const svg = await desktop().ValidateDiagram(staticSvg(drawing))
      onApply({
        scene: JSON.stringify({ type: 'excalidraw', version: 2, elements }),
        svg,
        caption: text,
      })
    } catch (problem) {
      setError(String(problem instanceof Error ? problem.message : problem))
    }
  }
  return (
    <div className="diagram-editor">
      <div className="diagram-canvas">
        <Excalidraw
          excalidrawAPI={setApi}
          theme={dark ? 'dark' : 'light'}
          initialData={{ elements: sceneElements(scene), scrollToContent: true }}
          UIOptions={{ tools: { image: false } }}
        />
      </div>
      <label className="field">
        <span>Caption</span>
        <input value={text} onChange={(event) => setText(event.target.value)} />
      </label>
      {error && <p role="alert">{error}</p>}
      <Button onClick={() => void apply()}>Apply diagram</Button>
    </div>
  )
}
