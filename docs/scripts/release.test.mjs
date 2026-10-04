import assert from 'node:assert/strict';
import test from 'node:test';
import remarkReleaseVersion from './remark-release-version.mjs';
import { latestRelease } from './resolve-release.mjs';
import { imageVersions } from './versions.mjs';

test('renders the selected version in prose, shell commands and environment examples', () => {
  const tree = { children: [
    { type: 'text', value: 'Latest: {{release_tag}}' },
    { type: 'code', value: 'release_tag="{{release_tag}}"\nBEATSTASH_VERSION="{{release_version}}"' },
  ] };
  remarkReleaseVersion({ tag: 'v2.10.3' })(tree);
  assert.equal(tree.children[0].value, 'Latest: v2.10.3');
  assert.equal(tree.children[1].value, 'release_tag="v2.10.3"\nBEATSTASH_VERSION="2.10.3"');
});

test('reads GitHub latest stable release', async () => {
  const tag = await latestRelease('example/beatstash', undefined, async (url) => {
    assert.equal(url, 'https://api.github.com/repos/example/beatstash/releases/latest');
    return { ok: true, json: async () => ({ tag_name: 'v2.10.3', draft: false, prerelease: false }) };
  });
  assert.equal(tag, 'v2.10.3');
});

test('allows a preview before the first release, but reports API failures', async () => {
  assert.equal(await latestRelease('example/beatstash', undefined, async () => ({ status: 404 })), '');
  await assert.rejects(latestRelease('example/beatstash', undefined, async () => ({ status: 503, ok: false })), /HTTP 503/);
});

test('rejects draft, prerelease and unsafe version values', async () => {
  for (const release of [{ tag_name: 'v1.2.3', draft: true }, { tag_name: 'v1.2.3-rc.1', prerelease: true }, { tag_name: 'v1.2.3\nINJECTED=value' }]) {
    await assert.rejects(latestRelease('example/beatstash', undefined, async () => ({ ok: true, json: async () => release })));
  }
});

test('renders placeholders that MDX parsed as expressions', () => {
  const tree = { children: [{ type: 'mdxTextExpression', value: '{release_tag}', data: { estree: {} } }] };
  remarkReleaseVersion({ tag: 'v2.10.3' })(tree);
  assert.deepEqual(tree.children[0], { type: 'text', value: 'v2.10.3' });
});

test('reads service versions from the deployment sample', () => {
  const versions = imageVersions('    image: postgres:18\n    image: deluan/navidrome:0.64.2\n');
  assert.deepEqual(versions, { '{{navidrome_version}}': '0.64.2', '{{postgres_version}}': '18' });
});
