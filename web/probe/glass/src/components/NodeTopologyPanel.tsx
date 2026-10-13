// NodeTopologyPanel.vue is an admin-only home tool. On Captain the public page is never
// logged in (the API adapter's getMe() always returns logged_in: false), so
// HomeView never offers it and it cannot render in either build; not ported.
export default function NodeTopologyPanel(_props: Record<string, unknown>) {
  return null
}
