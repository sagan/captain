import en from './i18n/en.json'
import zhCN from './i18n/zh-CN.json'
import zhTW from './i18n/zh-TW.json'
import ja from './i18n/ja.json'
import ko from './i18n/ko.json'
import ru from './i18n/ru.json'
const resources: Record<string, Record<string, string>> = {en,'zh-CN':zhCN,'zh-TW':zhTW,ja,ko,ru}
const requested = localStorage.getItem('i18nextLng') || navigator.language
export const language = resources[requested] ? requested : /^zh-(TW|HK|Hant)/i.test(requested) ? 'zh-TW' : requested.startsWith('zh') ? 'zh-CN' : resources[requested.split('-')[0]!] ? requested.split('-')[0]! : 'en'
document.documentElement.lang = language
export function t(source: string): string { return resources[language]?.[source] ?? source }
