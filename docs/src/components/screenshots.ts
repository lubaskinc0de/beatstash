import type { ImageMetadata } from 'astro';

const files = import.meta.glob<{ default: ImageMetadata }>('../assets/screenshots/*.{png,jpg,jpeg,webp}', { eager: true });

// Screenshots are looked up by file name without the extension, so a missing capture leaves its slot empty.
export function findScreenshot(name: string): ImageMetadata | undefined {
  return Object.entries(files).find(([path]) => path.split('/').pop()?.replace(/\.[^.]+$/, '') === name)?.[1].default;
}
