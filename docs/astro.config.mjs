import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import remarkDocLinks from './scripts/remark-doc-links.mjs';
import remarkReleaseVersion from './scripts/remark-release-version.mjs';
import { imageVersions } from './scripts/versions.mjs';

const repository = process.env.GITHUB_REPOSITORY || 'lubaskinc0de/beatstash';
const [owner, name] = repository.split('/');
const base = process.env.DOCS_BASE || (name === `${owner}.github.io` ? '/' : `/${name}`);
const releaseTag = process.env.DOCS_RELEASE_TAG;

export default defineConfig({
  site: process.env.DOCS_SITE || `https://${owner}.github.io`,
  base,
  trailingSlash: 'always',
  // Released installers print this address.
  redirects: { '/administration/https/': `${base.replace(/\/$/, '')}/installation/https/` },
  vite: { define: { __RELEASE_TAG__: JSON.stringify(releaseTag || 'vX.Y.Z') } },
  markdown: { remarkPlugins: [
    [remarkReleaseVersion, { tag: releaseTag, values: imageVersions() }],
    [remarkDocLinks, { base }],
  ] },
  integrations: [
    starlight({
      title: 'beatstash',
      description: 'Upload, import, and share your Navidrome music through Telegram.',
      locales: { root: { label: 'English', lang: 'en' } },
      social: [{ icon: 'github', label: 'GitHub', href: `https://github.com/${repository}` }],
      customCss: ['./src/styles/custom.css'],
      editLink: { baseUrl: `https://github.com/${repository}/edit/master/docs/` },
      lastUpdated: true,
      disable404Route: true,
      sidebar: [
        { label: 'Start here', items: ['index', 'introduction/comparison'] },
        { label: 'Install', items: ['installation/requirements', 'installation/new-server', 'installation/existing-navidrome', 'installation/telegram', 'installation/https'] },
        { label: 'Use beatstash', items: ['using/getting-started', 'using/uploading', 'using/listening', 'using/sharing', 'using/libraries'] },
        { label: 'Import and sync', items: ['import/sources', 'import/zvuk'] },
        { label: 'Run your server', items: ['administration/configuration', 'administration/participants', 'administration/storage-chat', 'administration/blocked-telegram', 'administration/updates', 'administration/uninstall', 'administration/troubleshooting'] },
        { label: 'Reference', items: ['reference/configuration', 'reference/glossary'] },
        { label: 'Contribute', items: ['development/local', 'development/contributing'] },
      ],
    }),
  ],
});
