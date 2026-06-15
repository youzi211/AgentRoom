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
      aria-orientation={direction === 'horizontal' ? 'vertical' : 'horizontal'}
      className={`resize-handle resize-handle--${direction}${isDragging ? ' resize-handle--active' : ''}`}
      onMouseDown={handleMouseDown}
      role="separator"
    />
  )
}
