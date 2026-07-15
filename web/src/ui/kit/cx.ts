// Собирает className из частей, отбрасывая пустые/ложные значения.
export function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(' ')
}
