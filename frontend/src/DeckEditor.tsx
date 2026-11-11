import { useEffect, useState } from 'react'
import { desktop } from './api'
import type { Attachment } from './api'
import type { Deck, Slide } from './artifact-model'
import { Button, Select } from './components/ui'
import { deckFont, deckLayout, deckPalette, slideSize, type SlideRect } from './deck-theme'

function area(rect: SlideRect) {
  return {
    left: `${(rect.x / slideSize.width) * 100}%`,
    top: `${(rect.y / slideSize.height) * 100}%`,
    width: `${(rect.w / slideSize.width) * 100}%`,
    height: `${(rect.h / slideSize.height) * 100}%`,
  }
}

function SlidePreview({ slide, number, image }: { slide: Slide; number: number; image?: string }) {
  const peak = Math.max(1, ...(slide.chart?.values ?? [1]))
  return (
    <div
      className={`slide-preview layout-${slide.layout}`}
      aria-label={`Slide ${number} preview`}
      style={{ background: `#${deckPalette.background}`, fontFamily: deckFont }}
    >
      <div
        className="slide-strip"
        style={{ ...area(deckLayout.strip), background: `#${deckPalette.accent}` }}
      />
      {slide.layout === 'title' ? (
        <>
          <h1 className="slide-title accent" style={area(deckLayout.titleSlide)}>
            {slide.title}
          </h1>
          <ul className="slide-subtitle" style={area(deckLayout.subtitle)}>
            {(slide.body ?? []).map((point) => (
              <li key={point}>{point}</li>
            ))}
          </ul>
        </>
      ) : (
        <>
          <h1 className="slide-title" style={area(deckLayout.title)}>
            {slide.title}
          </h1>
          {slide.layout === 'bullets' && (
            <ul className="slide-points" style={area(deckLayout.body)}>
              {(slide.body ?? []).map((point) => (
                <li key={point}>{point}</li>
              ))}
            </ul>
          )}
          {slide.layout === 'columns' && (
            <div className="slide-columns" style={area(deckLayout.body)}>
              <ul>
                {(slide.left ?? []).map((point) => (
                  <li key={point}>{point}</li>
                ))}
              </ul>
              <ul>
                {(slide.right ?? []).map((point) => (
                  <li key={point}>{point}</li>
                ))}
              </ul>
            </div>
          )}
          {slide.layout === 'image' && (
            <>
              {image && (
                <img className="slide-image" style={area(deckLayout.image)} src={image} alt="" />
              )}
              <p className="slide-caption" style={area(deckLayout.caption)}>
                {(slide.body ?? []).join(' · ')}
              </p>
            </>
          )}
          {slide.layout === 'chart' && slide.chart && (
            <div className="slide-chart" style={area(deckLayout.chart)}>
              {slide.chart.values.map((value, position) => (
                <div key={slide.chart!.labels[position]}>
                  <strong>{value}</strong>
                  <span
                    style={{
                      height: `${Math.max(2, (value / peak) * 100)}%`,
                      background: `#${deckPalette.chart[position % deckPalette.chart.length]}`,
                    }}
                  />
                  <small>{slide.chart!.labels[position]}</small>
                </div>
              ))}
            </div>
          )}
        </>
      )}
      <span className="slide-number">{number}</span>
    </div>
  )
}

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
        <SlidePreview
          slide={slide}
          number={index + 1}
          image={slide.layout === 'image' && preview ? preview : undefined}
        />
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
            <Select
              label="Layout"
              value={slide.layout}
              onChange={(layout) =>
                update({
                  layout,
                  ...(layout === 'chart' && !slide.chart
                    ? { chart: { labels: ['Build', 'Test'], values: [30, 20] } }
                    : {}),
                  ...(layout === 'image' ? { imageId: slide.imageId || images[0]?.id } : {}),
                })
              }
              options={['title', 'bullets', 'columns', 'image', 'chart'].map((layout) => ({
                value: layout,
                label: layout,
                disabled: layout === 'image' && images.length === 0,
              }))}
            />
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
                  <Select
                    label="Attached image"
                    value={slide.imageId ?? ''}
                    onChange={(imageId) => update({ imageId })}
                    options={images.map((image) => ({ value: image.id, label: image.name }))}
                  />
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
