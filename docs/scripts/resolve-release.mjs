import { appendFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { validateReleaseTag } from './remark-release-version.mjs';

export async function latestRelease(repository, token, request = fetch) {
  if (!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repository)) {
    throw new Error('Invalid GitHub repository');
  }
  const headers = { Accept: 'application/vnd.github+json' };
  if (token) headers.Authorization = `Bearer ${token}`;
  const response = await request(`https://api.github.com/repos/${repository}/releases/latest`, {
    headers,
    signal: AbortSignal.timeout(15000),
  });
  // A repository without a stable release can still preview its documentation.
  if (response.status === 404) return '';
  if (!response.ok) throw new Error(`Release lookup failed: HTTP ${response.status}`);
  const release = await response.json();
  if (release.draft || release.prerelease) throw new Error('Expected a published stable release');
  return validateReleaseTag(release.tag_name);
}

if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) {
  const repository = process.env.GITHUB_REPOSITORY || 'lubaskinc0de/beatstash';
  const tag = await latestRelease(repository, process.env.GH_TOKEN);
  if (process.env.GITHUB_ENV) appendFileSync(process.env.GITHUB_ENV, `DOCS_RELEASE_TAG=${tag}\n`);
  console.log(tag ? `Documentation release: ${tag}` : 'No stable release yet; using preview placeholders.');
}
