import { useEffect, useState } from 'react'
import { desktop } from './api'
import type { Attachment } from './api'
import type { Deck, Slide } from './artifact-model'
import { Button } from './components/ui'

export function DeckEditor({
  deck,
  chat,
  onChange,
}: {
  deck: Deck
  chat: string
  onChange(value: Deck): void
}) {
  const [requestedIndex, setIndex] = useState(0)
  const index = Math.min(requestedIndex, deck.slides.length - 1)
  const [images, setImages] = useState<Attachment[]>([])
  const [imagePreview, setPreview] = useState({ id: '', value: '' })
  const slide = deck.slides[index]
  const preview = imagePreview.id === slide.imageId ? imagePreview.value : ''
  useEffect(() => {
    void desktop()
      .Attachments(chat)
      .then((files) => setImages(files.filter((file) => file.mime.startsWith('image/'))))
      .catch(() => {})
  }, [chat])
  useEffect(() => {
    let active = true
    if (slide.imageId)
      void desktop()
        .AttachmentPreview(chat, slide.imageId)
        .then((value) => {
          if (active) setPreview({ id: slide.imageId!, value })
        })
        .catch(() => {})
    return () => {
      active = false
    }
  }, [chat, slide.imageId])
  const update = (patch: Partial<Slide>) => {
    const next = structuredClone(deck)
    next.slides[index] = { ...next.slides[index], ...patch }
    onChange(next)
  }
  const move = (offset: number) => {
    const target = index + offset
    if (target < 0 || target >= deck.slides.length) return
    const next = structuredClone(deck)
    ;[next.slides[index], next.slides[target]] = [next.slides[target], next.slides[index]]
    onChange(next)
    setIndex(target)
  }
  const lines = (value: string) => value.split('\n')
  return (
    <div className="deck-editor">
      <nav className="slide-list" aria-label="Slides">
        {deck.slides.map((item, i) => (
          <button
            key={item.id}
            aria-current={index === i ? 'true' : undefined}
            onClick={() => setIndex(i)}
          >
            <small>{i + 1}</small>
            <span>{item.title || 'Untitled slide'}</span>
          </button>
        ))}
        <Button
          variant="ghost"
          disabled={deck.slides.length >= 100}
          onClick={() => {
            const next = structuredClone(deck)
            next.slides.push({
              id: crypto.randomUUID(),
              layout: 'bullets',
              title: 'New slide',
              body: [],
            })
            onChange(next)
            setIndex(next.slides.length - 1)
          }}
        >
          Add slide
        </Button>
      </nav>
      <div className="slide-workspace">
        <div
          className={`slide-preview layout-${slide.layout}`}
          aria-label={`Slide ${index + 1} preview`}
        >
          <h1>{slide.title}</h1>
          {slide.layout === 'columns' ? (
            <div className="slide-columns">
              <ul>
                {slide.left?.map((text, i) => (
                  <li key={i}>{text}</li>
                ))}
              </ul>
              <ul>
                {slide.right?.map((text, i) => (
                  <li key={i}>{text}</li>
                ))}
              </ul>
            </div>
          ) : slide.layout === 'chart' && slide.chart ? (
            <div className="slide-chart">
              {slide.chart.labels.map((label, i) => {
                const maximum = Math.max(1, ...slide.chart!.values.map(Math.abs))
                return (
                  <div key={i}>
                    <strong>{slide.chart!.values[i]}</strong>
                    <span
                      style={{
                        height: `${Math.max(2, (Math.abs(slide.chart!.values[i]) / maximum) * 130)}px`,
                      }}
                    />
                    <small>{label}</small>
                  </div>
                )
              })}
            </div>
          ) : (
            <>
              {slide.layout === 'image' && preview && <img src={preview} alt={slide.title} />}
              <ul>
                {slide.body?.map((text, i) => (
                  <li key={i}>{text}</li>
                ))}
              </ul>
            </>
          )}
          <small className="slide-number">{index + 1}</small>
        </div>
        <form className="slide-fields" onSubmit={(event) => event.preventDefault()}>
          <label className="field">
            <span>Title</span>
            <input
              aria-label="Slide title"
              value={slide.title}
              maxLength={100}
              onChange={(event) => update({ title: event.target.value })}
            />
          </label>
          <label className="field">
            <span>Layout</span>
            <select
              value={slide.layout}
              onChange={(event) =>
                update({
                  layout: event.target.value,
                  ...(event.target.value === 'chart' && !slide.chart
                    ? { chart: { labels: ['Build', 'Test'], values: [30, 20] } }
                    : {}),
                  ...(event.target.value === 'image'
                    ? { imageId: slide.imageId || images[0]?.id }
                    : {}),
                })
              }
            >
              {['title', 'bullets', 'columns', 'image', 'chart'].map((layout) => (
                <option
                  key={layout}
                  value={layout}
                  disabled={layout === 'image' && images.length === 0}
                >
                  {layout}
                </option>
              ))}
            </select>
          </label>
          {slide.layout === 'columns' ? (
            <>
              <label className="field">
                <span>Left column · one point per line</span>
                <textarea
                  value={slide.left?.join('\n') ?? ''}
                  onChange={(event) => update({ left: lines(event.target.value) })}
                />
              </label>
              <label className="field">
                <span>Right column · one point per line</span>
                <textarea
                  value={slide.right?.join('\n') ?? ''}
                  onChange={(event) => update({ right: lines(event.target.value) })}
                />
              </label>
            </>
          ) : slide.layout === 'chart' ? (
            <div className="chart-fields">
              {slide.chart?.labels.map((label, i) => (
                <div key={i}>
                  <input
                    aria-label={`Chart label ${i + 1}`}
                    value={label}
                    maxLength={40}
                    onChange={(event) => {
                      const chart = structuredClone(slide.chart!)
                      chart.labels[i] = event.target.value
                      update({ chart })
                    }}
                  />
                  <input
                    aria-label={`Chart value ${i + 1}`}
                    type="number"
                    value={slide.chart?.values[i] ?? 0}
                    onChange={(event) => {
                      const chart = structuredClone(slide.chart!)
                      chart.values[i] = Number(event.target.value)
                      update({ chart })
                    }}
                  />
                </div>
              ))}
              <Button
                variant="ghost"
                disabled={(slide.chart?.labels.length ?? 0) >= 12}
                onClick={() =>
                  update({
                    chart: {
                      labels: [...(slide.chart?.labels ?? []), 'Item'],
                      values: [...(slide.chart?.values ?? []), 0],
                    },
                  })
                }
              >
                Add chart value
              </Button>
            </div>
          ) : (
            <>
              <label className="field">
                <span>Body · up to six short points</span>
                <textarea
                  value={slide.body?.join('\n') ?? ''}
                  onChange={(event) => update({ body: lines(event.target.value) })}
                />
              </label>
              {slide.layout === 'image' && (
                <label className="field">
                  <span>Attached image</span>
                  <select
                    value={slide.imageId ?? ''}
                    onChange={(event) => update({ imageId: event.target.value })}
                  >
                    {images.map((image) => (
                      <option key={image.id} value={image.id}>
                        {image.name}
                      </option>
                    ))}
                  </select>
                </label>
              )}
            </>
          )}
          <label className="field">
            <span>Speaker notes</span>
            <textarea
              value={slide.notes ?? ''}
              maxLength={10000}
              onChange={(event) => update({ notes: event.target.value })}
            />
          </label>
          <div className="slide-actions">
            <Button variant="ghost" disabled={index === 0} onClick={() => move(-1)}>
              Move earlier
            </Button>
            <Button
              variant="ghost"
              disabled={index === deck.slides.length - 1}
              onClick={() => move(1)}
            >
              Move later
            </Button>
            <Button
              variant="danger"
              disabled={deck.slides.length === 1}
              onClick={() => {
                if (!window.confirm('Delete this slide?')) return
                const next = structuredClone(deck)
                next.slides.splice(index, 1)
                onChange(next)
                setIndex(Math.max(0, index - 1))
              }}
            >
              Delete slide
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}
