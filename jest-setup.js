// Jest setup provided by Grafana scaffolding
import './.config/jest-setup';

global.IntersectionObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
};
HTMLCanvasElement.prototype.getContext = () => ({
  measureText: (text) => ({ width: String(text).length * 8 }),
});

// jsdom doesn't compute CSS display values from user-agent stylesheets.
// dom-accessibility-api uses getComputedStyle(el).getPropertyValue('display') to
// decide whether to insert a space separator between text nodes when computing
// accessible names. Without this patch, inline elements like <mark> and <span>
// get treated as block-level, breaking accessible name queries like getByRole('option', { name: /value/ }).
const _origGetComputedStyle = window.getComputedStyle.bind(window);
const _inlineElements = new Set([
  'A', 'ABBR', 'ACRONYM', 'B', 'BDO', 'BIG', 'BR', 'BUTTON', 'CITE', 'CODE',
  'DFN', 'EM', 'I', 'IMG', 'INPUT', 'KBD', 'LABEL', 'MAP', 'MARK', 'OUTPUT',
  'Q', 'SAMP', 'SELECT', 'SMALL', 'SPAN', 'STRONG', 'S', 'SUB', 'SUP',
  'TEXTAREA', 'TIME', 'TT', 'U', 'VAR',
]);
Object.defineProperty(window, 'getComputedStyle', {
  value: (element, pseudo) => {
    const style = _origGetComputedStyle(element, pseudo);
    if (!pseudo && element && element.tagName && _inlineElements.has(element.tagName)) {
      const existingDisplay = element.style && element.style.display;
      if (!existingDisplay) {
        return new Proxy(style, {
          get(target, prop) {
            if (prop === 'display') {
              return 'inline';
            }
            if (prop === 'getPropertyValue') {
              return (name) => (name === 'display' ? 'inline' : target.getPropertyValue(name));
            }
            const val = target[prop];
            return typeof val === 'function' ? val.bind(target) : val;
          },
        });
      }
    }
    return style;
  },
  writable: true,
  configurable: true,
});
