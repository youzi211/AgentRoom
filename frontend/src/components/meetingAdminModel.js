export const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: 'active', label: '进行中' },
  { value: 'closed', label: '已关闭' },
  { value: 'archived', label: '已归档' },
]

export const ROOM_LIFECYCLE_ACTIONS = {
  detail: 'detail',
  archive: 'archive',
  reopen: 'reopen',
  restore: 'restore',
}

export function actionsForRoomStatus(status = '') {
  switch (status) {
    case 'archived':
      return [ROOM_LIFECYCLE_ACTIONS.detail, ROOM_LIFECYCLE_ACTIONS.restore]
    case 'closed':
      return [ROOM_LIFECYCLE_ACTIONS.detail, ROOM_LIFECYCLE_ACTIONS.reopen, ROOM_LIFECYCLE_ACTIONS.archive]
    case 'active':
    default:
      return [ROOM_LIFECYCLE_ACTIONS.detail, ROOM_LIFECYCLE_ACTIONS.archive]
  }
}

export function labelForRoomStatus(status = '') {
  switch (status) {
    case 'closed':
      return '已关闭'
    case 'archived':
      return '已归档'
    case 'active':
    default:
      return '进行中'
  }
}

export function toneForRoomStatus(status = '') {
  switch (status) {
    case 'closed':
      return 'closed'
    case 'archived':
      return 'archived'
    case 'active':
    default:
      return 'active'
  }
}

export function labelForRoomAction(action = '') {
  switch (action) {
    case ROOM_LIFECYCLE_ACTIONS.archive:
      return '归档'
    case ROOM_LIFECYCLE_ACTIONS.reopen:
      return '恢复会议'
    case ROOM_LIFECYCLE_ACTIONS.restore:
      return '取消归档'
    case ROOM_LIFECYCLE_ACTIONS.detail:
    default:
      return '详情'
  }
}
