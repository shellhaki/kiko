import { Extension } from '@tiptap/core'
import { Plugin } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'
import type { Question } from './api'
export function questionHighlights(questions: { current: Question[] }) {
  return Extension.create({
    name: 'questions',
    addProseMirrorPlugins: () => [
      new Plugin({
        props: {
          decorations(state) {
            const decorations: Decoration[] = []
            state.doc.descendants((node, pos) => {
              const items = questions.current.filter(
                (q) => !q.resolved && q.blockId === node.attrs.id,
              )
              if (!items.length) return
              const positions: number[] = []
              node.descendants((child, offset) => {
                if (child.isText) {
                  for (let i = 0; i < (child.text?.length ?? 0); i++)
                    positions.push(pos + 1 + offset + i)
                } else if (child.type.name === 'hardBreak') {
                  positions.push(pos + 1 + offset)
                }
              })
              for (const q of items) {
                const text = node.textContent
                const start = Array.from(text).slice(0, q.start).join('').length
                const end = Array.from(text).slice(0, q.end).join('').length
                if (positions[start] !== undefined && positions[end - 1] !== undefined)
                  decorations.push(
                    Decoration.inline(positions[start], positions[end - 1] + 1, {
                      class: 'question-anchor',
                      'data-question': q.id,
                    }),
                  )
              }
            })
            return DecorationSet.create(state.doc, decorations)
          },
        },
      }),
    ],
  })
}
