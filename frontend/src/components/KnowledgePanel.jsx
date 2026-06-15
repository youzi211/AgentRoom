import { useEffect, useRef, useState } from 'react'

function KnowledgePanel({ description, disabled = false, emptyText, listDocuments, onDeleteDocument, onUploadDocument, title }) {
  const [documents, setDocuments] = useState([])
  const [isLoading, setIsLoading] = useState(false)
  const [isUploading, setIsUploading] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')
  const fileInputRef = useRef(null)

  useEffect(() => {
    let isCurrent = true

    const loadDocuments = async () => {
      if (disabled || !listDocuments) {
        setDocuments([])
        return
      }

      setIsLoading(true)
      setErrorMessage('')
      try {
        const response = await listDocuments()
        if (isCurrent) {
          setDocuments(response.documents ?? [])
        }
      } catch (error) {
        if (isCurrent) {
          setErrorMessage(error.message || '加载知识文档失败。')
        }
      } finally {
        if (isCurrent) {
          setIsLoading(false)
        }
      }
    }

    void loadDocuments()

    return () => {
      isCurrent = false
    }
  }, [disabled, listDocuments])

  const handleFileChange = async (event) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file || disabled) {
      return
    }

    if (!file.name.toLowerCase().endsWith('.md')) {
      setErrorMessage('当前只支持上传 .md 文件。')
      return
    }

    setIsUploading(true)
    setErrorMessage('')
    try {
      const response = await onUploadDocument(file)
      setDocuments((current) => [response.document, ...current])
    } catch (error) {
      setErrorMessage(error.message || '上传知识文档失败。')
    } finally {
      setIsUploading(false)
    }
  }

  const handleDelete = async (documentId) => {
    if (disabled || !onDeleteDocument) {
      return
    }

    setErrorMessage('')
    try {
      await onDeleteDocument(documentId)
      setDocuments((current) => current.filter((document) => document.id !== documentId))
    } catch (error) {
      setErrorMessage(error.message || '删除知识文档失败。')
    }
  }

  return (
    <section className="sidebar-section knowledge-panel">
      <div className="sidebar-header">
        <div>
          <h2>{title}</h2>
          {description ? <p className="sidebar-note">{description}</p> : null}
        </div>
        <span className="sidebar-count">{documents.length}</span>
      </div>

      <div className="knowledge-actions">
        <input ref={fileInputRef} type="file" accept=".md,.markdown,text/markdown,text/plain" hidden onChange={handleFileChange} />
        <button
          className="button button--secondary button--compact"
          type="button"
          onClick={() => fileInputRef.current?.click()}
          disabled={disabled || isUploading}
        >
          {isUploading ? '上传中...' : '上传 .md'}
        </button>
      </div>

      {errorMessage ? <p className="knowledge-error">{errorMessage}</p> : null}

      {isLoading ? (
        <p className="sidebar-empty">正在加载知识文档...</p>
      ) : documents.length === 0 ? (
        <p className="sidebar-empty">{emptyText}</p>
      ) : (
        <ul className="knowledge-list">
          {documents.map((document) => (
            <li className="knowledge-list-item" key={document.id}>
              <div className="knowledge-document-main">
                <span className="knowledge-document-name">{document.fileName}</span>
                <span className="knowledge-document-meta">
                  {formatFileSize(document.sizeBytes)} - {formatDate(document.createdAt)}
                </span>
              </div>
              {onDeleteDocument ? (
                <button
                  className="knowledge-delete-button"
                  type="button"
                  title="删除知识文档"
                  onClick={() => handleDelete(document.id)}
                  disabled={disabled || isUploading}
                >
                  删除
                </button>
              ) : null}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

function formatFileSize(value) {
  const bytes = Number(value)
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return '0 KB'
  }
  if (bytes < 1024 * 1024) {
    return `${Math.max(1, Math.round(bytes / 1024))} KB`
  }
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function formatDate(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return ''
  }
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: 'numeric',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

export default KnowledgePanel
