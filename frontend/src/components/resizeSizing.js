export function calculateResizeSize({
  currentPosition,
  invertDelta = false,
  maxSize,
  minSize,
  startPosition,
  startSize,
}) {
  const pointerDelta = currentPosition - startPosition
  const sizeDelta = invertDelta ? -pointerDelta : pointerDelta
  let nextSize = startSize + sizeDelta

  if (Number.isFinite(minSize)) {
    nextSize = Math.max(nextSize, minSize)
  }

  if (Number.isFinite(maxSize)) {
    nextSize = Math.min(nextSize, maxSize)
  }

  return nextSize
}
