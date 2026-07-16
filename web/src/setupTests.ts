import '@testing-library/jest-dom'

// jsdom не реализует matchMedia; motion его использует для prefers-reduced-motion.
// В тестах включаем reduced-motion → анимации мгновенные, вывод чистый.
if (!window.matchMedia) {
  window.matchMedia = (query: string) =>
    ({
      matches: query.includes('prefers-reduced-motion'),
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }) as MediaQueryList
}
