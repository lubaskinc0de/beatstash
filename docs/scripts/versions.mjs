import { readFileSync } from 'node:fs';

// The deployment sample is the source of truth for the service versions the docs name.
export function imageVersions(compose = readFileSync(new URL('../../deploy/compose.yml', import.meta.url), 'utf8')) {
  const tag = (image) => {
    const match = compose.match(new RegExp(`image: ${image.replace('/', '\\/')}:(\\S+)`));
    if (!match) throw new Error(`No ${image} image in deploy/compose.yml`);
    return match[1];
  };
  return {
    '{{navidrome_version}}': tag('deluan/navidrome'),
    '{{postgres_version}}': tag('postgres'),
  };
}
