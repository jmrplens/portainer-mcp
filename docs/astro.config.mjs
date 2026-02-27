// @ts-check
import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

// https://astro.build/config
export default defineConfig({
  site: "https://portainer.github.io",
  base: "/portainer-mcp",
  integrations: [
    starlight({
      title: "Portainer MCP",
      description:
        "Model Context Protocol server for Portainer container management",
      social: [
        {
          icon: "github",
          label: "GitHub",
          href: "https://github.com/portainer/portainer-mcp",
        },
      ],
      editLink: {
        baseUrl:
          "https://github.com/portainer/portainer-mcp/edit/main/docs/",
      },
      sidebar: [
        {
          label: "Home",
          link: "/",
        },
        {
          label: "Getting Started",
          items: [
            { label: "Quick Start", slug: "getting-started" },
            { label: "Configuration", slug: "configuration" },
          ],
        },
        {
          label: "Guides",
          items: [
            { label: "Meta-Tools", slug: "guides/meta-tools" },
            { label: "Security", slug: "guides/security" },
            { label: "Troubleshooting & FAQ", slug: "guides/troubleshooting" },
          ],
        },
        {
          label: "Reference",
          items: [
            { label: "Tools Reference", slug: "reference/api-reference" },
            { label: "Architecture", slug: "reference/architecture" },
            { label: "Clients & Models", slug: "reference/clients-and-models" },
            { label: "Design Decisions", slug: "reference/design-decisions" },
          ],
        },
        {
          label: "Development",
          items: [
            { label: "Contributing", slug: "development/contributing" },
            {
              label: "Project Structure",
              slug: "development/project-structure",
            },
            {
              label: "Development Workflow",
              slug: "development/workflow",
            },
            { label: "Adding Tools", slug: "development/adding-tools" },
            { label: "Testing Guide", slug: "development/testing" },
            {
              label: "Dependencies & Toolchain",
              slug: "development/dependencies",
            },
            { label: "CI/CD & Releases", slug: "development/ci-cd" },
          ],
        },
      ],
      customCss: ["./src/styles/custom.css"],
      lastUpdated: true,
    }),
  ],
});
