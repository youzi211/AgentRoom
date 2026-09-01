import { useCallback, useEffect, useRef, useState } from 'react'
import { calculateResizeSize } from './resizeSizing'

export default function ResizeHandle({
  direction = 'horizontal',
  invertDelta = false,
  maxHeight,
  maxWidth,
  minHeight,
  minWidth = 150,
  onResize,
  size,
}) {
  const [isDragging, setIsDragging] = useState(false)
  const startPosRef = useRef(0)
  const startSizeRef = useRef(0)
  const handleRef = useRef(null)

  const handleMouseDown = useCallback((e) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(true)
    
    const pos = direction === 'horizontal' ? e.clientX : e.clientY
    startPosRef.current = pos

    if (Number.isFinite(size)) {
      startSizeRef.current = size
      return
    }

    const target = handleRef.current?.previousElementSibling
    if (target) {
      startSizeRef.current = direction === 'horizontal' ? target.offsetWidth : target.offsetHeight
    }
  }, [direction, size])

  const handleKeyDown = useCallback((e) => {
    const key = e.key
    if (key !== 'ArrowLeft' && key !== 'ArrowRight' && key !== 'ArrowUp' && key !== 'ArrowDown') {
      return
    }
    e.preventDefault()
    const step = e.shiftKey ? 32 : 8
    const isHorizontal = direction === 'horizontal'
    const delta = key === 'ArrowLeft' || key === 'ArrowDown' ? -step : step
    const signedDelta = invertDelta ? -delta : delta
    const min = isHorizontal ? minWidth : (minHeight || minWidth)
    const max = isHorizontal ? maxWidth : (maxHeight || maxWidth)
    const newSize = calculateResizeSize({
      currentPosition: 0,
      invertDelta,
      maxSize: max,
      minSize: min,
      startPosition: 0,
      startSize: size,
    }) + signedDelta
    const clamped = Math.max(min, Math.min(max ?? Infinity, newSize))
    onResize?.(clamped)
  }, [direction, invertDelta, minWidth, maxWidth, minHeight, maxHeight, size, onResize])

  useEffect(() => {
    if (!isDragging) return

    const handleMouseMove = (e) => {
      e.preventDefault()
      const currentPos = direction === 'horizontal' ? e.clientX : e.clientY
      const min = direction === 'horizontal' ? minWidth : (minHeight || minWidth)
      const max = direction === 'horizontal' ? maxWidth : (maxHeight || maxWidth)

      const newSize = calculateResizeSize({
        currentPosition: currentPos,
        invertDelta,
        maxSize: max,
        minSize: min,
        startPosition: startPosRef.current,
        startSize: startSizeRef.current,
      })

      onResize?.(newSize)
    }

    const handleMouseUp = () => {
      setIsDragging(false)
    }

    document.addEventListener('mousemove', handleMouseMove, { passive: false })
    document.addEventListener('mouseup', handleMouseUp)
    document.body.style.cursor = direction === 'horizontal' ? 'col-resize' : 'row-resize'
    document.body.style.userSelect = 'none'

    return () => {
      document.removeEventListener('mousemove', handleMouseMove)
      document.removeEventListener('mouseup', handleMouseUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }
  }, [isDragging, direction, invertDelta, minWidth, maxWidth, minHeight, maxHeight, onResize])

  return (
    <div
      ref={handleRef}
      aria-label="调整面板大小"
      aria-orientation={direction === 'horizontal' ? 'vertical' : 'horizontal'}
      className={`resize-handle resize-handle--${direction}${isDragging ? ' resize-handle--active' : ''}`}
      onKeyDown={handleKeyDown}
      onMouseDown={handleMouseDown}
      role="separator"
      tabIndex={0}
    />
  )
}
