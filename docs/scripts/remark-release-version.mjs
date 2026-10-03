const stableTag = /^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)$/;

export function validateReleaseTag(tag) {
  if (!stableTag.test(tag)) throw new Error(`Invalid stable release tag: ${tag}`);
  return tag;
}

export default function remarkReleaseVersion({ tag }) {
  if (tag) validateReleaseTag(tag);
  const values = {
    '{{release_tag}}': tag || 'vX.Y.Z',
    '{{release_version}}': tag ? tag.slice(1) : 'X.Y.Z',
  };
  return (tree) => {
    function visit(node) {
      if (typeof node.value === 'string') {
        for (const [placeholder, value] of Object.entries(values)) {
          node.value = node.value.replaceAll(placeholder, value);
        }
      }
      for (const child of node.children || []) visit(child);
    }
    visit(tree);
  };
}
