# Kestra source provenance

Source: https://github.com/kestra-io/kestra/tree/v2.0.7/ui/packages/design-system/src
Commit: 49b62a2c820d37e98a1c7b63aa35fd83f960d4ec
License: Apache-2.0, see LICENSE. Upstream source notices retained.

The imported component dependency closure is copied unchanged; unrelated components, tests and stories are excluded. Keel adapters live outside this directory. Only explicitly imported components are bundled. This is the native design-system foundation, not a claim that the entire Kestra UI has been ported.

`RunTimeline.vue` is adapted from `ui/src/components/executions/overview/components/RunTimeline.vue` at the same pinned commit. Modifications are identified in its file header: imports, local types/date formatter and non-teleported popover. Design-system dependencies remain upstream source.
