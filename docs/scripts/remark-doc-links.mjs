import { existsSync } from 'node:fs';
import { dirname, relative, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const docsRoot = fileURLToPath(new URL('../src/content/docs/', import.meta.url));

// Keep Markdown links usable in a checkout and render them as site routes.
export default function remarkDocLinks({ base }) {
  return (tree, file) => {
    function visit(node) {
      if (['link', 'definition'].includes(node.type) && /^\.{1,2}\//.test(node.url)) {
        const [path, hash] = node.url.split('#');
        if (/\.mdx?$/.test(path)) {
          const target = resolve(dirname(file.path), path);
          const local = relative(docsRoot, target);
          if (local.startsWith(`..${sep}`) || !existsSync(target)) {
            throw new Error(`Invalid documentation link in ${file.path}: ${node.url}`);
          }
          const slug = local.replaceAll(sep, '/').replace(/\.mdx?$/, '').replace(/(^|\/)index$/, '');
          node.url = `${base.replace(/\/$/, '')}/${slug}${slug ? '/' : ''}${hash ? `#${hash}` : ''}`;
        }
      }
      for (const child of node.children || []) visit(child);
    }
    visit(tree);
  };
}
