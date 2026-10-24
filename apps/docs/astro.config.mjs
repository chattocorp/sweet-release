import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  integrations: [
    starlight({
      title: 'Sweet Release',
      description: 'Release packages with Markdown change files.',
      favicon: '/favicon.svg',
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/chattocorp/sweet-release' },
      ],
      editLink: {
        baseUrl: 'https://github.com/chattocorp/sweet-release/edit/main/apps/docs/',
      },
      customCss: ['./src/styles/custom.css'],
      sidebar: [
        { label: 'Overview', slug: 'index' },
        { label: 'Getting started', slug: 'getting-started' },
        {
          label: 'Guides',
          items: [
            { label: 'Write release notes', slug: 'guides/release-notes' },
            { label: 'Prepare a release', slug: 'guides/prepare-a-release' },
            { label: 'GitHub Actions', slug: 'guides/github-actions' },
          ],
        },
        { label: 'Configuration', slug: 'reference/configuration' },
      ],
    }),
  ],
});
