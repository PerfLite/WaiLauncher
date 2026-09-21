// @ts-check
import { Events, Browser, Window, Clipboard } from '@wailsio/runtime';

/**
 * EventsOn registers an event listener that passes unwrapped data payload to callback
 * @param {string} eventName
 * @param {(data: any) => void} callback
 */
export function EventsOn(eventName, callback) {
    return Events.On(eventName, (ev) => {
        if (ev && typeof ev === 'object' && 'data' in ev) {
            callback(ev.data);
        } else {
            callback(ev);
        }
    });
}

/**
 * EventsOnce registers a one-time event listener
 * @param {string} eventName
 * @param {(data: any) => void} callback
 */
export function EventsOnce(eventName, callback) {
    return Events.Once(eventName, (ev) => {
        if (ev && typeof ev === 'object' && 'data' in ev) {
            callback(ev.data);
        } else {
            callback(ev);
        }
    });
}

/**
 * EventsOnMultiple registers an event listener for N invocations
 * @param {string} eventName
 * @param {(data: any) => void} callback
 * @param {number} maxCallbacks
 */
export function EventsOnMultiple(eventName, callback, maxCallbacks) {
    return Events.OnMultiple(eventName, (ev) => {
        if (ev && typeof ev === 'object' && 'data' in ev) {
            callback(ev.data);
        } else {
            callback(ev);
        }
    }, maxCallbacks);
}

export function EventsOff(...eventNames) {
    return Events.Off(...eventNames);
}

export function EventsOffAll() {
    return Events.OffAll();
}

export function EventsEmit(eventName, ...data) {
    return Events.Emit(eventName, data.length > 1 ? data : data[0]);
}

export function BrowserOpenURL(url) {
    return Browser.OpenURL(url);
}

export function ClipboardGetText() {
    return Clipboard.Text();
}

export function ClipboardSetText(text) {
    return Clipboard.SetText(text);
}

export function WindowMinimise() {
    return Window.Minimise();
}

export function WindowUnminimise() {
    return Window.UnMinimise();
}

export function WindowToggleMaximise() {
    return Window.ToggleMaximise();
}

export function WindowClose() {
    return Window.Close();
}