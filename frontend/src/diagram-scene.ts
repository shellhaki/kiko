import { convertToExcalidrawElements } from '@excalidraw/excalidraw'

export function sceneElements(scene: string) {
  const { skeleton, elements } = scene ? JSON.parse(scene) : { elements: [] }
  return skeleton
    ? convertToExcalidrawElements(
        elements.map((element: object) => ({ roughness: 0, fontFamily: 2, ...element })),
      )
    : elements
}
export function staticSvg(svg: SVGSVGElement) {
  const clone = svg.cloneNode(true) as SVGSVGElement
  clone.querySelectorAll('style, metadata, mask').forEach((node) => node.remove())
  for (const node of [clone, ...clone.querySelectorAll('*')])
    for (const name of ['style', 'direction', 'version', 'xmlns']) node.removeAttribute(name)
  return new XMLSerializer().serializeToString(clone)
}
