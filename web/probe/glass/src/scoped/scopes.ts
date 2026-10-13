// Scope attributes (and v-bind CSS variable names) of the compiled Vue scoped
// styles in ./*.css, which came from the upstream theme's SFCs. Spread
// `{ [scope.attr]: '' }` on every element a component renders, exactly where
// Vue would add its data-v attribute.
export const scopes = {
  "background": {
    "attr": "data-v-background",
    "vars": {
      "defaultBackgroundImage": "--v121a3af0"
    }
  },
  "loading-cover": {
    "attr": "data-v-loading-cover",
    "vars": {}
  },
  "markdown-renderer": {
    "attr": "data-v-markdown-renderer",
    "vars": {}
  },
  "node-card": {
    "attr": "data-v-node-card",
    "vars": {}
  },
  "node-earth-cobe-globe": {
    "attr": "data-v-node-earth-cobe-globe",
    "vars": {}
  },
  "node-earth-realistic-globe": {
    "attr": "data-v-node-earth-realistic-globe",
    "vars": {}
  },
  "node-earth-tiled-map": {
    "attr": "data-v-node-earth-tiled-map",
    "vars": {}
  },
  "node-general-cards": {
    "attr": "data-v-node-general-cards",
    "vars": {}
  },
  "node-list": {
    "attr": "data-v-node-list",
    "vars": {}
  },
  "traffic-progress": {
    "attr": "data-v-traffic-progress",
    "vars": {}
  },
  "visitor-info": {
    "attr": "data-v-visitor-info",
    "vars": {}
  },
  "home-view": {
    "attr": "data-v-home-view",
    "vars": {}
  }
} as const
