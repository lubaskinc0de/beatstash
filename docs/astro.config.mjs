import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import remarkDocLinks from './scripts/remark-doc-links.mjs';
import remarkReleaseVersion from './scripts/remark-release-version.mjs';

const repository = process.env.GITHUB_REPOSITORY || 'lubaskinc0de/beatstash';
const [owner, name] = repository.split('/');
const base = name === `${owner}.github.io` ? '/' : `/${name}`;

export default defineConfig({
  site: process.env.DOCS_SITE || `https://${owner}.github.io`,
  base: process.env.DOCS_BASE || base,
  trailingSlash: 'always',
  markdown: { remarkPlugins: [
    [remarkReleaseVersion, { tag: process.env.DOCS_RELEASE_TAG }],
    [remarkDocLinks, { base: process.env.DOCS_BASE || base }],
  ] },
  integrations: [
    starlight({
      title: 'beatstash',
      description: 'Upload, import, and share your Navidrome music through Telegram.',
      locales: { root: { label: 'English', lang: 'en' } },
      social: [{ icon: 'github', label: 'GitHub', href: `https://github.com/${repository}` }],
      customCss: ['./src/styles/custom.css'],
      disable404Route: true,
      sidebar: [
        { label: 'Start here', items: ['index', 'introduction/comparison', 'installation/existing-navidrome', 'installation/new-server', 'installation/telegram'] },
        { label: 'Use beatstash', items: ['using/getting-started', 'using/uploading', 'using/listening', 'using/sharing', 'using/libraries'] },
        { label: 'Import and sync', items: ['import/sources', 'import/zvuk'] },
        { label: 'Run your server', items: ['administration/configuration', 'administration/https', 'administration/updates', 'administration/troubleshooting'] },
        { label: 'Contribute', items: ['development/local', 'development/contributing'] },
      ],
    }),
  ],
});
