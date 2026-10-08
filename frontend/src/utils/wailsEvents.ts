import { Events } from '@wailsio/runtime'

/**
 * Subscribes to a Wails event, automatically extracting ev.data for the callback.
 * Returns an unsubscribe function.
 */
export function onEvent<T = any>(eventName: string, callback: (data: T) => void): () => void {
  return Events.On(eventName, (ev: any) => {
    if (ev && typeof ev === 'object' && 'data' in ev) {
      callback(ev.data as T)
    } else {
      callback(ev as T)
    }
  })
}

/**
 * Subscribes to a Wails event once, automatically extracting ev.data for the callback.
 */
export function onceEvent<T = any>(eventName: string, callback: (data: T) => void): () => void {
  return Events.Once(eventName, (ev: any) => {
    if (ev && typeof ev === 'object' && 'data' in ev) {
      callback(ev.data as T)
    } else {
      callback(ev as T)
    }
  })
}

/**
 * Emits a Wails event.
 */
export function emitEvent<T = any>(eventName: string, data?: T): Promise<boolean> {
  return Events.Emit(eventName, data)
}
