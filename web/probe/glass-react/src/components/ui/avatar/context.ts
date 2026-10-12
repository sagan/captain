import { createContext } from 'react'

export type ImageLoadingStatus = 'idle' | 'loading' | 'loaded' | 'error'

// reka 的 AvatarRoot 上下文：AvatarImage 写入、AvatarFallback 读取。
export interface AvatarState {
  status: ImageLoadingStatus
  setStatus: (status: ImageLoadingStatus) => void
}

export const AvatarStateContext = createContext<AvatarState>({ status: 'idle', setStatus: () => {} })
