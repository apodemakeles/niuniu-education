// 统一的 API 请求封装。
// baseUrl 由环境变量 VITE_API_BASE 注入：开发期 /api/v1（走 vite proxy），部署期绝对地址。

const BASE = import.meta.env.VITE_API_BASE || '/api/v1'

export class ApiError extends Error {
  code: string
  status: number
  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

interface ApiOptions extends Omit<RequestInit, 'headers'> {
  query?: Record<string, string | number | undefined>
  headers?: Record<string, string>
}

function buildUrl(path: string, query?: ApiOptions['query']): string {
  const url = `${BASE}${path}`
  if (!query) return url
  const params = new URLSearchParams()
  for (const [k, v] of Object.entries(query)) {
    if (v !== undefined && v !== '') params.set(k, String(v))
  }
  const qs = params.toString()
  return qs ? `${url}?${qs}` : url
}

export async function request<T>(path: string, opts: ApiOptions = {}): Promise<T> {
  const { query, headers, ...rest } = opts
  // FormData 时让浏览器自动设置 Content-Type（含 boundary），不手动覆盖
  const isFormData = rest.body instanceof FormData
  const finalHeaders: Record<string, string> = { ...headers }
  if (!isFormData) {
    finalHeaders['Content-Type'] = 'application/json'
  }
  let res: Response
  try {
    res = await fetch(buildUrl(path, query), {
      headers: finalHeaders,
      ...rest,
    })
  } catch (e) {
    throw new ApiError(0, 'NETWORK', `无法连接到服务：${(e as Error).message}`)
  }

  const text = await res.text()
  const data = text ? JSON.parse(text) : null

  if (!res.ok) {
    const errBody = data?.error
    throw new ApiError(res.status, errBody?.code || 'HTTP_ERROR', errBody?.message || `请求失败 (${res.status})`)
  }
  return data as T
}

// --- SSE 流式消费 ---

export interface SSEEvent {
  event: string
  data: any
}

/**
 * 流式 SSE 请求。用 fetch + ReadableStream 消费 Server-Sent Events。
 * （不能用 EventSource，因为它不支持 POST + multipart）
 *
 * @param path API 路径
 * @param opts fetch 选项（body/method/headers）
 * @param onEvent 每收到一个 SSE 事件的回调
 * @param signal 可选的 AbortSignal，用于取消
 */
export async function streamSSE(
  path: string,
  opts: RequestInit,
  onEvent: (ev: SSEEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const url = `${BASE}${path}`
  let res: Response
  try {
    res = await fetch(url, { ...opts, signal })
  } catch (e) {
    if ((e as Error).name === 'AbortError') return
    throw new ApiError(0, 'NETWORK', `无法连接到服务：${(e as Error).message}`)
  }

  if (!res.ok) {
    const text = await res.text().catch(() => '')
    const errBody = text ? JSON.parse(text)?.error : null
    throw new ApiError(res.status, errBody?.code || 'HTTP_ERROR', errBody?.message || `请求失败 (${res.status})`)
  }

  const reader = res.body!.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })

    // SSE 事件以空行分隔（\n\n）
    const parts = buffer.split('\n\n')
    buffer = parts.pop()! // 最后一段可能不完整，保留

    for (const part of parts) {
      if (!part.trim()) continue
      const ev = parseSSEBlock(part)
      if (ev) onEvent(ev)
    }
  }
  // 处理残留
  if (buffer.trim()) {
    const ev = parseSSEBlock(buffer)
    if (ev) onEvent(ev)
  }
}

// 解析一个 SSE 事件块（形如 "event: row\ndata: {...}"）
function parseSSEBlock(block: string): SSEEvent | null {
  let event = 'message'
  let dataStr = ''
  for (const line of block.split('\n')) {
    if (line.startsWith('event:')) {
      event = line.slice(6).trim()
    } else if (line.startsWith('data:')) {
      dataStr += line.slice(5).trim()
    }
  }
  if (!dataStr) return null
  try {
    return { event, data: JSON.parse(dataStr) }
  } catch {
    return { event, data: dataStr }
  }
}
