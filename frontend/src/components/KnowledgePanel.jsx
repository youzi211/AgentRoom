import { useEffect, useRef, useState } from 'react'
import { Alert, Badge, Button, Group, Paper, Stack, Text, Title } from '@mantine/core'

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
    <Paper component="section" className="sidebar-section knowledge-panel" withBorder radius="md" shadow="none">
      <div className="sidebar-header">
        <div>
          <Title order={2}>{title}</Title>
          {description ? <Text className="sidebar-note">{description}</Text> : null}
        </div>
        <Badge className="sidebar-count" color="teal" variant="light">{documents.length}</Badge>
      </div>

      <Group className="knowledge-actions" gap="xs">
        <input ref={fileInputRef} type="file" accept=".md,.markdown,text/markdown,text/plain" hidden onChange={handleFileChange} />
        <Button
          variant="light"
          color="teal"
          size="xs"
          type="button"
          onClick={() => fileInputRef.current?.click()}
          disabled={disabled || isUploading}
        >
          {isUploading ? '上传中...' : '上传 .md'}
        </Button>
      </Group>

      {errorMessage ? <Alert color="red" variant="light">{errorMessage}</Alert> : null}

      {isLoading ? (
        <Text className="sidebar-empty">正在加载知识文档...</Text>
      ) : documents.length === 0 ? (
        <Text className="sidebar-empty">{emptyText}</Text>
      ) : (
        <Stack component="ul" className="knowledge-list" gap="xs">
          {documents.map((document) => (
            <Paper component="li" className="knowledge-list-item" key={document.id} withBorder radius="md" shadow="none">
              <div className="knowledge-document-main">
                <Text className="knowledge-document-name">{document.fileName}</Text>
                <Text className="knowledge-document-meta">
                  {formatFileSize(document.sizeBytes)} - {formatDate(document.createdAt)}
                </Text>
              </div>
              {onDeleteDocument ? (
                <Button
                  className="knowledge-delete-button"
                  type="button"
                  title="删除知识文档"
                  variant="subtle"
                  color="red"
                  size="xs"
                  onClick={() => handleDelete(document.id)}
                  disabled={disabled || isUploading}
                >
                  删除
                </Button>
              ) : null}
            </Paper>
          ))}
        </Stack>
      )}
    </Paper>
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
