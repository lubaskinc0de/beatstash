const stableTag = /^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)$/;

export function validateReleaseTag(tag) {
  if (!stableTag.test(tag)) throw new Error(`Invalid stable release tag: ${tag}`);
  return tag;
}

export default function remarkReleaseVersion({ tag, values: extra = {} }) {
  if (tag) validateReleaseTag(tag);
  const values = {
    '{{release_tag}}': tag || 'vX.Y.Z',
    '{{release_version}}': tag ? tag.slice(1) : 'X.Y.Z',
    ...extra,
  };
  return (tree) => {
    function visit(node) {
      // MDX parses `{{name}}` in prose as an expression `{name}`; turn it back into text.
      if (node.type === 'mdxTextExpression' && `{${node.value}}` in values) {
        node.type = 'text';
        node.value = values[`{${node.value}}`];
        delete node.data;
      } else if (typeof node.value === 'string') {
        for (const [placeholder, value] of Object.entries(values)) {
          node.value = node.value.replaceAll(placeholder, value);
        }
      }
      for (const child of node.children || []) visit(child);
    }
    visit(tree);
  };
}
