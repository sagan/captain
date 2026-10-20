export interface NodeCost { amount: string; currency: string; cycle: string; years?: string }
const words: Record<string, string[]> = {
  en: ['Free', 'month', 'quarter', '6 months', 'year', 'years'],
  'zh-CN': ['免费', '月', '季度', '半年', '年', '年'],
  'zh-TW': ['免費', '月', '季度', '半年', '年', '年'],
  ja: ['無料', '月', '四半期', '半年', '年', '年'],
  ko: ['무료', '월', '분기', '반년', '년', '년'],
  ru: ['Бесплатно', 'месяц', 'квартал', 'полгода', 'год', 'лет'],
}
export function nodePrice(info: { price?: string; billing?: NodeCost }, locale = 'en'): string {
  const b = info.billing
  if (!b) return info.price ?? ''
  if (!b.amount) return ''
  const labels = words[locale] ?? words[locale.split('-')[0]] ?? words.en
  if (Number(b.amount) === 0) return labels[0]
  const cycles: Record<string, string> = { monthly: labels[1], quarterly: labels[2], semiannual: labels[3], annual: labels[4] }
  const cycle = b.cycle === 'custom' ? `${b.years} ${labels[5]}` : cycles[b.cycle]
  return `${b.currency} ${b.amount} / ${cycle ?? b.cycle}`
}
