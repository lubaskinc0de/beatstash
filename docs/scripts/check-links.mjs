import { readdir, readFile, stat } from 'node:fs/promises';
import { resolve, relative } from 'node:path';

const root = resolve('dist');
const [owner, name] = (process.env.GITHUB_REPOSITORY || 'lubaskinc0de/beatstash').split('/');
const base = (process.env.DOCS_BASE || (name === `${owner}.github.io` ? '/' : `/${name}`)).replace(/\/$/, '');
const origin = process.env.DOCS_SITE || `https://${owner}.github.io`;

async function walk(dir) {
  const entries = await readdir(dir, { withFileTypes: true });
  const files = await Promise.all(entries.map((entry) => {
    const path = resolve(dir, entry.name);
    return entry.isDirectory() ? walk(path) : [path];
  }));
  return files.flat();
}

const files = (await walk(root)).filter((path) => path.endsWith('.html'));
const html = new Map(await Promise.all(files.map(async (path) => [path, await readFile(path, 'utf8')])));
const failures = [];
let checked = 0;

for (const [path, text] of html) {
  const route = relative(root, path).replaceAll('\\', '/').replace(/index\.html$/, '');
  const page = new URL(`${base}/${route}`, origin);
  for (const match of text.matchAll(/\b(?:href|src)="([^"]+)"/g)) {
    const href = match[1].replaceAll('&amp;', '&');
    const url = new URL(href, page);
    if (url.origin !== page.origin || !['http:', 'https:'].includes(url.protocol)) continue;
    checked++;
    if (base && url.pathname !== base && !url.pathname.startsWith(`${base}/`)) {
      failures.push(`${route}: link escapes base path: ${href}`);
      continue;
    }
    let target = resolve(root, `.${decodeURIComponent(url.pathname.slice(base.length) || '/')}`);
    if (url.pathname === `${base}/404/`) target = resolve(root, '404.html');
    try {
      if ((await stat(target)).isDirectory()) target = resolve(target, 'index.html');
      await stat(target);
      if (url.hash && html.has(target)) {
        const id = decodeURIComponent(url.hash.slice(1));
        const escaped = id.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
        if (!new RegExp(`\\bid="${escaped}"`).test(html.get(target))) {
          failures.push(`${route}: missing anchor: ${href}`);
        }
      }
    } catch {
      failures.push(`${route}: missing target: ${href}`);
    }
  }
}

if (failures.length) {
  console.error(failures.join('\n'));
  process.exitCode = 1;
} else {
  console.log(`Checked ${files.length} pages and ${checked} internal links/assets.`);
}
