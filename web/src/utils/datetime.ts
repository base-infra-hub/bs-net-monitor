// 解析服务端返回的 ISO/RFC3339 时间，按浏览器本地时区格式化展示

const pad = (n: number) => String(n).padStart(2, '0')

const parseISO = (iso: string | undefined | null): Date | null => {
  if (!iso) return null
  const d = new Date(iso)
  return isNaN(d.getTime()) ? null : d
}

export const fmtDateTime = (iso?: string | null): string => {
  const d = parseISO(iso)
  if (!d) return iso ?? '--'
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

export const fmtDate = (iso?: string | null): string => {
  const d = parseISO(iso)
  if (!d) return iso ?? '--'
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

export const fmtTime = (iso?: string | null): string => {
  const d = parseISO(iso)
  if (!d) return iso ?? '--'
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

export const fmtHM = (iso?: string | null): string => {
  const d = parseISO(iso)
  if (!d) return iso ?? '--'
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export const fmtMD = (iso?: string | null): string => {
  const d = parseISO(iso)
  if (!d) return iso ?? '--'
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

export const fmtMDHM = (iso?: string | null): string => {
  const d = parseISO(iso)
  if (!d) return iso ?? '--'
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}
