import { defineEcConfig } from '@astrojs/starlight/expressive-code';

const tone = (dark, light) => ({ theme }) => (theme.type === 'dark' ? dark : light);

export default defineEcConfig({
  themes: ['everforest-dark', 'everforest-light'],
  styleOverrides: {
    borderRadius: '0.6rem',
    codeFontFamily: "'JetBrains Mono Variable', monospace",
    // Match the site's green-tinted surfaces instead of Everforest's own backgrounds.
    codeBackground: tone('#1f2925', '#f5faf7'),
    borderColor: tone('#27322e', '#dde7e2'),
    frames: {
      editorBackground: tone('#1f2925', '#f5faf7'),
      terminalBackground: tone('#1f2925', '#f5faf7'),
      editorTabBarBackground: tone('#25302b', '#eaf2ee'),
      editorActiveTabBackground: tone('#1f2925', '#f5faf7'),
      editorActiveTabIndicatorBottomColor: tone('#3bb08f', '#0c7a63'),
      terminalTitlebarBackground: tone('#25302b', '#eaf2ee'),
      frameBoxShadowCssValue: 'none',
    },
  },
});
